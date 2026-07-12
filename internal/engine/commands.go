package engine

import (
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
)

// CreateGame initializes a brand-new save with the given name, generating
// its galaxy, per-system markets, and starting player state.
type CreateGame struct {
	Name string
}

func (CreateGame) isCommand() {}

// Move flies the player's ship along the warp lane to To, spending the
// lane's turn cost.
type Move struct {
	To galaxy.NodeID
}

func (Move) isCommand() {}

// Buy purchases Quantity units of Commodity at the player's current system.
type Buy struct {
	Commodity economy.CommodityID
	Quantity  int
}

func (Buy) isCommand() {}

// Sell sells Quantity units of Commodity at the player's current system.
type Sell struct {
	Commodity economy.CommodityID
	Quantity  int
}

func (Sell) isCommand() {}
