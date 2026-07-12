package engine

import (
	"context"

	"github.com/rdu90/RPG/internal/engine/economy"
)

// getMarket returns commodity prices at the player's current system.
func (e *Engine) getMarket(ctx context.Context) ([]economy.Price, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return nil, err
	}
	return e.repo.GetMarket(ctx, p.NodeID)
}
