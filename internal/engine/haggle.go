package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"time"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/haggle"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
	"github.com/rdu90/RPG/internal/rng"
)

const (
	reputationFloor = -50
	reputationCeil  = 50
)

// HaggleResult carries a negotiation's current state alongside the player's
// current persisted state, so a round that concludes (accept/reject) shows
// its effects immediately without a second query.
type HaggleResult struct {
	Session haggle.Session
	Player  player.Player
}

// startHaggle opens a new negotiation over c.Quantity units of c.Commodity
// at the player's current system.
func (e *Engine) startHaggle(ctx context.Context, c StartHaggle) (HaggleResult, error) {
	if c.Quantity <= 0 {
		return HaggleResult{}, fmt.Errorf("engine: quantity must be positive")
	}

	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return HaggleResult{}, err
	}

	prices, err := e.repo.GetMarket(ctx, p.NodeID)
	if err != nil {
		return HaggleResult{}, err
	}
	price, ok := findPrice(prices, c.Commodity)
	if !ok {
		return HaggleResult{}, fmt.Errorf("engine: commodity %s is not traded at %s", c.Commodity, p.NodeID)
	}

	if c.Buying {
		if p.CargoUsed()+c.Quantity > p.CargoCapacity {
			return HaggleResult{}, fmt.Errorf("engine: insufficient cargo space: need %d, have %d free",
				c.Quantity, p.CargoCapacity-p.CargoUsed())
		}
	} else if p.Cargo[c.Commodity] < c.Quantity {
		return HaggleResult{}, fmt.Errorf("engine: insufficient cargo: need %d, have %d", c.Quantity, p.Cargo[c.Commodity])
	}

	gal, err := e.repo.GetGalaxy(ctx)
	if err != nil {
		return HaggleResult{}, err
	}
	node, ok := gal.Node(p.NodeID)
	if !ok {
		return HaggleResult{}, fmt.Errorf("engine: unknown system %s", p.NodeID)
	}

	game, err := e.repo.GetGame(ctx)
	if err != nil {
		return HaggleResult{}, err
	}

	disp := haggle.NewDisposition(node.DevelopmentLevel, p.ReputationAt(p.NodeID))
	r := rng.New(haggleSeed(game.ID, p.NodeID, c.Commodity, c.Buying, 0))
	session := haggle.Start(p.NodeID, c.Commodity, c.Buying, price, c.Quantity, disp, r)

	return HaggleResult{Session: session, Player: p}, nil
}

func (e *Engine) haggleOffer(ctx context.Context, c HaggleOffer) (HaggleResult, error) {
	return e.haggleRound(ctx, c.Session, true, func(r *rand.Rand, s haggle.Session) haggle.Session {
		return s.Offer(c.Price, r)
	})
}

func (e *Engine) haggleWalkAway(ctx context.Context, c HaggleWalkAway) (HaggleResult, error) {
	return e.haggleRound(ctx, c.Session, true, func(r *rand.Rand, s haggle.Session) haggle.Session {
		return s.WalkAway(r)
	})
}

func (e *Engine) haggleAccept(ctx context.Context, c HaggleAccept) (HaggleResult, error) {
	return e.haggleRound(ctx, c.Session, false, func(_ *rand.Rand, s haggle.Session) haggle.Session {
		return s.Accept()
	})
}

// haggleRound advances an in-progress Session by one action, optionally
// spending a turn for it (per the turn model, most actions cost turns; a
// haggling round does, accepting a standing offer doesn't), then settles the
// trade and reputation if the action concluded the negotiation.
func (e *Engine) haggleRound(ctx context.Context, s haggle.Session, spendTurn bool, advance func(*rand.Rand, haggle.Session) haggle.Session) (HaggleResult, error) {
	if s.Outcome != haggle.InProgress {
		return HaggleResult{}, fmt.Errorf("engine: haggle session has already concluded")
	}

	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return HaggleResult{}, err
	}

	if spendTurn {
		turns, err := p.Turns.Spend(time.Now().UTC(), 1)
		if err != nil {
			return HaggleResult{}, err
		}
		p.Turns = turns
	}

	game, err := e.repo.GetGame(ctx)
	if err != nil {
		return HaggleResult{}, err
	}
	r := rng.New(haggleSeed(game.ID, s.NodeID, s.Commodity, s.Buying, s.Round))

	s = advance(r, s)

	if s.Outcome != haggle.InProgress {
		if s.Outcome == haggle.Accepted {
			if err := settleHaggle(&p, s); err != nil {
				return HaggleResult{}, err
			}
		}
		if p.Reputation == nil {
			p.Reputation = map[galaxy.NodeID]int{}
		}
		rep := p.Reputation[s.NodeID] + s.ReputationDelta
		if rep < reputationFloor {
			rep = reputationFloor
		} else if rep > reputationCeil {
			rep = reputationCeil
		}
		p.Reputation[s.NodeID] = rep
	}

	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return HaggleResult{}, err
	}
	return HaggleResult{Session: s, Player: p}, nil
}

// settleHaggle applies an Accepted session's trade to p: credits/cargo move
// at the session's negotiated price, and the trade nudges the player's
// derived alignment toward the commodity's category.
func settleHaggle(p *player.Player, s haggle.Session) error {
	if s.Buying {
		cost := s.NPCOffer * s.Quantity
		if cost > p.Credits {
			return fmt.Errorf("engine: insufficient credits: need %d, have %d", cost, p.Credits)
		}
		if p.CargoUsed()+s.Quantity > p.CargoCapacity {
			return fmt.Errorf("engine: insufficient cargo space: need %d, have %d free",
				s.Quantity, p.CargoCapacity-p.CargoUsed())
		}
		p.Credits -= cost
		p.Cargo[s.Commodity] += s.Quantity
	} else {
		if p.Cargo[s.Commodity] < s.Quantity {
			return fmt.Errorf("engine: insufficient cargo: need %d, have %d", s.Quantity, p.Cargo[s.Commodity])
		}
		p.Credits += s.NPCOffer * s.Quantity
		p.Cargo[s.Commodity] -= s.Quantity
		if p.Cargo[s.Commodity] == 0 {
			delete(p.Cargo, s.Commodity)
		}
	}

	if c, ok := economy.Find(s.Commodity); ok {
		p.Alignment = p.Alignment.Nudge(player.ContributionFor(c.Category))
	}
	return nil
}

// haggleSeed derives a deterministic per-round RNG seed from the save's
// GameID and the negotiation's identifying details, so replaying the exact
// same round of the exact same session always resolves the same way without
// the engine needing to persist any RNG state between commands.
func haggleSeed(gameID ports.GameID, nodeID galaxy.NodeID, commodity economy.CommodityID, buying bool, round int) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(gameID))
	_, _ = h.Write([]byte(nodeID))
	_, _ = h.Write([]byte(commodity))
	if buying {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write([]byte{byte(round)})
	return int64(h.Sum64())
}

func findPrice(prices []economy.Price, id economy.CommodityID) (int, bool) {
	for _, p := range prices {
		if p.CommodityID == id {
			return p.Price, true
		}
	}
	return 0, false
}
