package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/rdu90/RPG/internal/engine/colony"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/player"
)

const (
	// colonizeBaseCost and colonizeCostPerDevelopmentLevel derive the
	// credit cost of founding a colony: more developed systems are pricier,
	// more contested real estate.
	colonizeBaseCost                = 800
	colonizeCostPerDevelopmentLevel = 400

	// colonizeTurnCost is the turn price of the settling action itself,
	// separate from the credit cost.
	colonizeTurnCost = 5
)

// ColonizeCost returns the credit cost of founding a colony at a system of
// the given development level.
func ColonizeCost(developmentLevel int) int {
	return colonizeBaseCost + colonizeCostPerDevelopmentLevel*developmentLevel
}

// ColonizeTurnCost is the turn price of founding a colony, independent of
// its credit cost.
const ColonizeTurnCost = colonizeTurnCost

// ColonizeResult is the result of a Colonize command: the newly-founded
// colony, alongside the player's current state.
type ColonizeResult struct {
	Player player.Player
	Colony colony.Colony
}

// ColonyStatus is the result of GetColony: whether a colony exists at the
// player's current system, and its state if so.
type ColonyStatus struct {
	Exists bool
	Colony colony.Colony
}

// colonize founds a colony at the player's current system.
func (e *Engine) colonize(ctx context.Context, c Colonize) (ColonizeResult, error) {
	if _, ok := economy.Find(c.Focus); !ok {
		return ColonizeResult{}, fmt.Errorf("engine: unknown commodity %q", c.Focus)
	}

	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return ColonizeResult{}, err
	}

	if _, ok, err := e.repo.GetColony(ctx, p.NodeID); err != nil {
		return ColonizeResult{}, err
	} else if ok {
		return ColonizeResult{}, fmt.Errorf("engine: a colony already exists at %s", p.NodeID)
	}

	gal, err := e.repo.GetGalaxy(ctx)
	if err != nil {
		return ColonizeResult{}, err
	}
	node, ok := gal.Node(p.NodeID)
	if !ok {
		return ColonizeResult{}, fmt.Errorf("engine: unknown system %s", p.NodeID)
	}

	cost := ColonizeCost(node.DevelopmentLevel)
	if p.Credits < cost {
		return ColonizeResult{}, fmt.Errorf("engine: need %d credits to found a colony here, have %d", cost, p.Credits)
	}

	turns, err := p.Turns.Spend(time.Now().UTC(), colonizeTurnCost)
	if err != nil {
		return ColonizeResult{}, err
	}
	p.Turns = turns
	p.Credits -= cost

	col := colony.New(p.NodeID, c.Focus, time.Now().UTC())
	if err := e.repo.SaveColony(ctx, col); err != nil {
		return ColonizeResult{}, err
	}
	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return ColonizeResult{}, err
	}

	return ColonizeResult{Player: p, Colony: col}, nil
}

// getColony returns the colony (if any) at the player's current system,
// advancing it to now first.
func (e *Engine) getColony(ctx context.Context) (ColonyStatus, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return ColonyStatus{}, err
	}

	col, ok, err := e.repo.GetColony(ctx, p.NodeID)
	if err != nil {
		return ColonyStatus{}, err
	}
	if !ok {
		return ColonyStatus{}, nil
	}

	col, err = e.tickColony(ctx, col)
	if err != nil {
		return ColonyStatus{}, err
	}
	return ColonyStatus{Exists: true, Colony: col}, nil
}

// getColonies returns every colony in the save, each advanced to now.
func (e *Engine) getColonies(ctx context.Context) ([]colony.Colony, error) {
	cols, err := e.repo.GetColonies(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]colony.Colony, 0, len(cols))
	for _, col := range cols {
		col, err := e.tickColony(ctx, col)
		if err != nil {
			return nil, err
		}
		out = append(out, col)
	}
	return out, nil
}

// tickColony advances c to now (population growth toward its system's
// development-level cap), and if any ticks elapsed, decays its Focus
// commodity's local market price by the same number of steps and persists
// both — the mechanism by which colony production feeds back into markets.
func (e *Engine) tickColony(ctx context.Context, c colony.Colony) (colony.Colony, error) {
	gal, err := e.repo.GetGalaxy(ctx)
	if err != nil {
		return colony.Colony{}, err
	}
	node, ok := gal.Node(c.NodeID)
	if !ok {
		return c, nil
	}

	updated, ticks := c.Ticked(time.Now().UTC(), node.DevelopmentLevel)
	if ticks == 0 {
		return updated, nil
	}

	if commodity, ok := economy.Find(c.Focus); ok {
		prices, err := e.repo.GetMarket(ctx, c.NodeID)
		if err != nil {
			return colony.Colony{}, err
		}
		for i, pr := range prices {
			if pr.CommodityID == c.Focus {
				prices[i].Price = colony.DecayedPrice(pr.Price, commodity.BasePrice, ticks)
				break
			}
		}
		if err := e.repo.SaveMarket(ctx, c.NodeID, prices); err != nil {
			return colony.Colony{}, err
		}
	}

	if err := e.repo.SaveColony(ctx, updated); err != nil {
		return colony.Colony{}, err
	}
	return updated, nil
}
