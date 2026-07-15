package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/rdu90/RPG/internal/engine/colony"
	"github.com/rdu90/RPG/internal/engine/combat"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/faction"
	"github.com/rdu90/RPG/internal/engine/fleet"
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

	startingAttack  = 12
	startingDefense = 6
	startingHull    = 50

	// rivalColonyMinDevelopmentLevel and rivalColonyChance gate which
	// systems can be seeded with a rival-faction colony at galaxy
	// generation: only reasonably developed systems, and only a fraction
	// of those, so a 16-node galaxy ends up with a handful of rival
	// colonies rather than one on every eligible system.
	rivalColonyMinDevelopmentLevel = 3
	rivalColonyChance              = 0.4

	// rivalColonyStartingPopulationFraction is how much of a rival
	// colony's development-level population cap it starts at, so it reads
	// as an already-established settlement rather than a fresh founding.
	rivalColonyStartingPopulationFraction = 0.5
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
	startNode := gal.Nodes[0].ID
	for _, node := range gal.Nodes {
		prices := economy.GenerateMarket(r, node.DevelopmentLevel)
		if err := e.repo.SaveMarket(ctx, node.ID, prices); err != nil {
			return ports.Game{}, fmt.Errorf("engine: save market for %s: %w", node.ID, err)
		}

		if node.ID == startNode || node.DevelopmentLevel < rivalColonyMinDevelopmentLevel {
			continue
		}
		if r.Float64() >= rivalColonyChance {
			continue
		}
		owner := faction.Catalog[r.IntN(len(faction.Catalog))].ID
		focus := economy.Commodities[r.IntN(len(economy.Commodities))].ID
		population := int(float64(colony.PopulationCap(node.DevelopmentLevel)) * rivalColonyStartingPopulationFraction)
		garrison := combat.GenerateGarrison(r, node.DevelopmentLevel)
		col := colony.NewRival(node.ID, focus, owner, population, garrison, game.CreatedAt)
		if err := e.repo.SaveColony(ctx, col); err != nil {
			return ports.Game{}, fmt.Errorf("engine: seed rival colony at %s: %w", node.ID, err)
		}
	}

	p := player.Player{
		Credits:       startingCredits,
		NodeID:        gal.Nodes[0].ID,
		CargoCapacity: cargoCapacity,
		Cargo:         map[economy.CommodityID]int{},
		Turns:         turn.New(turnsMax, turnRefillEvery, game.CreatedAt),
		Reputation:    map[galaxy.NodeID]int{},
		Discovered:    map[galaxy.NodeID]bool{gal.Nodes[0].ID: true},
		Ship:          fleet.Stats{Attack: startingAttack, Defense: startingDefense, Hull: startingHull, MaxHull: startingHull},
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
