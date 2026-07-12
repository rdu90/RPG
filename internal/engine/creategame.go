package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
	"github.com/rdu90/RPG/internal/engine/turn"
	"github.com/rdu90/RPG/internal/rng"
)

const (
	galaxySize      = 16
	startingCredits = 500
	cargoCapacity   = 40
	turnsMax        = 100
	turnRefillEvery = 20 * time.Second
)

// createGame creates the save's identity record, then generates and
// persists everything a new save needs to be playable: the galaxy, each
// system's market, and the player's starting state. The galaxy is seeded
// from the save's own ID, so it's reproducible but unique per save.
func (e *Engine) createGame(ctx context.Context, c CreateGame) (ports.Game, error) {
	game, err := e.repo.CreateGame(ctx, c.Name)
	if err != nil {
		return ports.Game{}, err
	}

	seed := seedFromID(game.ID)
	gal := galaxy.Generate(seed, galaxySize)
	if err := e.repo.SaveGalaxy(ctx, gal); err != nil {
		return ports.Game{}, fmt.Errorf("engine: save galaxy: %w", err)
	}

	r := rng.New(seed)
	for _, node := range gal.Nodes {
		prices := economy.GenerateMarket(r, node.DevelopmentLevel)
		if err := e.repo.SaveMarket(ctx, node.ID, prices); err != nil {
			return ports.Game{}, fmt.Errorf("engine: save market for %s: %w", node.ID, err)
		}
	}

	p := player.Player{
		Credits:       startingCredits,
		NodeID:        gal.Nodes[0].ID,
		CargoCapacity: cargoCapacity,
		Cargo:         map[economy.CommodityID]int{},
		Turns:         turn.New(turnsMax, turnRefillEvery, game.CreatedAt),
		Reputation:    map[galaxy.NodeID]int{},
	}
	if err := e.repo.InitPlayer(ctx, p); err != nil {
		return ports.Game{}, fmt.Errorf("engine: init player: %w", err)
	}

	return game, nil
}

// seedFromID derives a deterministic int64 seed from a save's GameID.
func seedFromID(id ports.GameID) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	return int64(h.Sum64())
}
