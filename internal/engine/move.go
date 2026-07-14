package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/rdu90/RPG/internal/engine/combat"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
	"github.com/rdu90/RPG/internal/rng"
)

// MoveResult carries the player's state after flying to a new system,
// alongside any hostile encountered on arrival — nil if the flight was
// uneventful. A non-nil Encounter must be resolved with ResolveEncounter
// before the player can act further.
type MoveResult struct {
	Player    player.Player
	Encounter *combat.Hostile
}

// move flies the player along the warp lane to c.To, spending the lane's
// turn cost, then rolls for a hostile encounter on arrival.
func (e *Engine) move(ctx context.Context, c Move) (MoveResult, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return MoveResult{}, err
	}

	gal, err := e.repo.GetGalaxy(ctx)
	if err != nil {
		return MoveResult{}, err
	}

	edge, ok := gal.EdgeBetween(p.NodeID, c.To)
	if !ok {
		return MoveResult{}, fmt.Errorf("engine: no warp lane from %s to %s", p.NodeID, c.To)
	}

	turns, err := p.Turns.Spend(time.Now().UTC(), edge.TurnCost)
	if err != nil {
		return MoveResult{}, err
	}
	p.Turns = turns
	p.NodeID = c.To
	p.Trips++
	if p.Discovered == nil {
		p.Discovered = map[galaxy.NodeID]bool{}
	}
	p.Discovered[c.To] = true

	node, ok := gal.Node(c.To)
	if !ok {
		return MoveResult{}, fmt.Errorf("engine: unknown system %s", c.To)
	}
	game, err := e.repo.GetGame(ctx)
	if err != nil {
		return MoveResult{}, err
	}

	var encounter *combat.Hostile
	seed := encounterSeed(game.ID, c.To, p.Trips)
	r := rng.New(seed)
	if r.Float64() < combat.EncounterChance(node.DevelopmentLevel) {
		h := combat.Generate(r, seed, node.DevelopmentLevel)
		encounter = &h
	}

	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return MoveResult{}, err
	}
	return MoveResult{Player: p, Encounter: encounter}, nil
}

// encounterSeed derives a deterministic per-arrival RNG seed from the
// save's GameID, the destination system, and the player's running trip
// count (the nonce that keeps repeated visits to the same system from
// always rolling the same encounter).
func encounterSeed(gameID ports.GameID, nodeID galaxy.NodeID, trips int) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(gameID))
	_, _ = h.Write([]byte("encounter"))
	_, _ = h.Write([]byte(nodeID))
	_, _ = h.Write([]byte(strconv.Itoa(trips)))
	return int64(h.Sum64())
}
