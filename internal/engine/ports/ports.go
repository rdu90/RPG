// Package ports defines the repository interfaces the engine depends on.
// Concrete implementations (e.g. internal/persistence/sqlite) satisfy these
// interfaces; the engine never imports a persistence package directly.
package ports

import (
	"context"
	"time"
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
