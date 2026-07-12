// Package engine is the pure game-logic core: it has no knowledge of any
// transport (TUI, network) and no knowledge of any specific persistence
// technology. Engine.Execute and Engine.Query are the only entry points
// callers use, so a future network transport can wrap the exact same
// Command/Query values without any change to this package.
package engine

import (
	"context"
	"fmt"

	"github.com/rdu90/RPG/internal/engine/ports"
)

// Command is anything that mutates game state via Engine.Execute.
type Command interface{ isCommand() }

// Query is anything that reads game state via Engine.Query.
type Query interface{ isQuery() }

// Engine is the composition root for game-logic subsystems. Subsystem
// repositories are added here as milestones bring them online.
type Engine struct {
	games ports.GameRepository
}

// New builds an Engine over the given repositories.
func New(games ports.GameRepository) *Engine {
	return &Engine{games: games}
}

// Execute dispatches a Command to the subsystem that handles it.
func (e *Engine) Execute(ctx context.Context, cmd Command) (any, error) {
	switch c := cmd.(type) {
	case CreateGame:
		return e.games.CreateGame(ctx, c.Name)
	default:
		return nil, fmt.Errorf("engine: unhandled command %T", cmd)
	}
}

// Query dispatches a Query to the subsystem that handles it.
func (e *Engine) Query(ctx context.Context, q Query) (any, error) {
	switch q.(type) {
	case GetGame:
		return e.games.GetGame(ctx)
	default:
		return nil, fmt.Errorf("engine: unhandled query %T", q)
	}
}
