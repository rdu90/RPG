// Package query re-exports the engine's Query vocabulary and result types
// under a transport-facing import path, for the same reason as
// internal/transport/command: callers above the transport boundary never
// import internal/engine directly.
package query

import (
	"github.com/rdu90/RPG/internal/engine"
	"github.com/rdu90/RPG/internal/engine/ports"
)

// GetGame returns the current save's identity record.
type GetGame = engine.GetGame

// Game is the result type returned for GetGame and for a successful
// CreateGame command.
type Game = ports.Game
