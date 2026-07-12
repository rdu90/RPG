// Package ports defines the repository interfaces the engine depends on.
// Concrete implementations (e.g. internal/persistence/sqlite) satisfy these
// interfaces; the engine never imports a persistence package directly.
package ports

import (
	"context"
	"time"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
)

// GameID identifies a save game.
type GameID string

// Game is the identity/metadata record for a single save. Each save is its
// own persistence store, so there is exactly one Game row per store.
type Game struct {
	ID        GameID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GameRepository persists and retrieves the save's identity record.
type GameRepository interface {
	// CreateGame initializes a brand-new save with the given name.
	CreateGame(ctx context.Context, name string) (Game, error)
	// GetGame returns the current save's identity record.
	GetGame(ctx context.Context) (Game, error)
}

// GalaxyRepository persists and retrieves the save's generated galaxy graph.
type GalaxyRepository interface {
	SaveGalaxy(ctx context.Context, g galaxy.Galaxy) error
	GetGalaxy(ctx context.Context) (galaxy.Galaxy, error)
}

// MarketRepository persists and retrieves per-system commodity prices.
type MarketRepository interface {
	SaveMarket(ctx context.Context, nodeID galaxy.NodeID, prices []economy.Price) error
	GetMarket(ctx context.Context, nodeID galaxy.NodeID) ([]economy.Price, error)
}

// PlayerRepository persists and retrieves the player's ship/economy state.
type PlayerRepository interface {
	// InitPlayer creates the save's single player row.
	InitPlayer(ctx context.Context, p player.Player) error
	GetPlayer(ctx context.Context) (player.Player, error)
	SavePlayer(ctx context.Context, p player.Player) error
}

// Repository is the full set of repositories a persistence backend must
// implement. A save is a single backend implementing all of them.
type Repository interface {
	GameRepository
	GalaxyRepository
	MarketRepository
	PlayerRepository
}
