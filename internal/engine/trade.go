package engine

import (
	"context"
	"fmt"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/player"
)

// trade buys or sells qty units of commodity id at the player's current
// system, at that system's static market price.
func (e *Engine) trade(ctx context.Context, id economy.CommodityID, qty int, buy bool) (player.Player, error) {
	if qty <= 0 {
		return player.Player{}, fmt.Errorf("engine: quantity must be positive")
	}

	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return player.Player{}, err
	}

	prices, err := e.repo.GetMarket(ctx, p.NodeID)
	if err != nil {
		return player.Player{}, err
	}

	price, ok := findPrice(prices, id)
	if !ok {
		return player.Player{}, fmt.Errorf("engine: commodity %s is not traded at %s", id, p.NodeID)
	}

	if buy {
		cost := price * qty
		if cost > p.Credits {
			return player.Player{}, fmt.Errorf("engine: insufficient credits: need %d, have %d", cost, p.Credits)
		}
		if p.CargoUsed()+qty > p.CargoCapacity {
			return player.Player{}, fmt.Errorf("engine: insufficient cargo space: need %d, have %d free",
				qty, p.CargoCapacity-p.CargoUsed())
		}
		p.Credits -= cost
		p.Cargo[id] += qty
	} else {
		if p.Cargo[id] < qty {
			return player.Player{}, fmt.Errorf("engine: insufficient cargo: need %d, have %d", qty, p.Cargo[id])
		}
		p.Credits += price * qty
		p.Cargo[id] -= qty
		if p.Cargo[id] == 0 {
			delete(p.Cargo, id)
		}
	}

	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return player.Player{}, err
	}
	return p, nil
}

func findPrice(prices []economy.Price, id economy.CommodityID) (int, bool) {
	for _, p := range prices {
		if p.CommodityID == id {
			return p.Price, true
		}
	}
	return 0, false
}
