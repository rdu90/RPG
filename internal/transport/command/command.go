// Package command re-exports the engine's Command vocabulary under a
// transport-facing import path. Callers above the transport boundary (the
// TUI, and later a network client) depend on this package instead of
// internal/engine directly, so the engine package can be swapped for a
// remote implementation without touching caller code.
package command

import "github.com/rdu90/RPG/internal/engine"

// CreateGame starts a new save with the given name.
type CreateGame = engine.CreateGame

// Move flies the player's ship to an adjacent system.
type Move = engine.Move

// Buy purchases cargo at the player's current system.
type Buy = engine.Buy

// Sell sells cargo at the player's current system.
type Sell = engine.Sell
