// Package player models the player's persistent ship/economy state:
// position in the galaxy, credits, cargo hold, and turn allowance.
package player

import (
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/turn"
)

// Player is a save's single player state.
type Player struct {
	Credits       int
	NodeID        galaxy.NodeID
	CargoCapacity int
	Cargo         map[economy.CommodityID]int
	Turns         turn.Allowance
}

// CargoUsed returns the total units currently held across all commodities.
func (p Player) CargoUsed() int {
	used := 0
	for _, qty := range p.Cargo {
		used += qty
	}
	return used
}
