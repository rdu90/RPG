package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
)

// move flies the player along the warp lane to c.To, spending the lane's
// turn cost.
func (e *Engine) move(ctx context.Context, c Move) (player.Player, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return player.Player{}, err
	}

	gal, err := e.repo.GetGalaxy(ctx)
	if err != nil {
		return player.Player{}, err
	}

	edge, ok := gal.EdgeBetween(p.NodeID, c.To)
	if !ok {
		return player.Player{}, fmt.Errorf("engine: no warp lane from %s to %s", p.NodeID, c.To)
	}

	turns, err := p.Turns.Spend(time.Now().UTC(), edge.TurnCost)
	if err != nil {
		return player.Player{}, err
	}
	p.Turns = turns
	p.NodeID = c.To
	if p.Discovered == nil {
		p.Discovered = map[galaxy.NodeID]bool{}
	}
	p.Discovered[c.To] = true

	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return player.Player{}, err
	}
	return p, nil
}
