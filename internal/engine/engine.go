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

// Engine is the composition root for game-logic subsystems, backed by a
// single repository that persists every subsystem's state for one save.
type Engine struct {
	repo ports.Repository
}

// New builds an Engine over the given repository.
func New(repo ports.Repository) *Engine {
	return &Engine{repo: repo}
}

// Execute dispatches a Command to the subsystem that handles it.
func (e *Engine) Execute(ctx context.Context, cmd Command) (any, error) {
	switch c := cmd.(type) {
	case CreateGame:
		return e.createGame(ctx, c)
	case Move:
		return e.move(ctx, c)
	case Buy:
		return e.trade(ctx, c.Commodity, c.Quantity, true)
	case Sell:
		return e.trade(ctx, c.Commodity, c.Quantity, false)
	default:
		return nil, fmt.Errorf("engine: unhandled command %T", cmd)
	}
}

// Query dispatches a Query to the subsystem that handles it.
func (e *Engine) Query(ctx context.Context, q Query) (any, error) {
	switch q.(type) {
	case GetGame:
		return e.repo.GetGame(ctx)
	case GetGalaxy:
		return e.repo.GetGalaxy(ctx)
	case GetPlayer:
		return e.repo.GetPlayer(ctx)
	case GetMarket:
		return e.getMarket(ctx)
	default:
		return nil, fmt.Errorf("engine: unhandled query %T", q)
	}
}
