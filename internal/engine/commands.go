package engine

import (
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/haggle"
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

// StartHaggle opens a negotiation over Quantity units of Commodity at the
// player's current system: Buying true negotiates a purchase, false a sale.
type StartHaggle struct {
	Commodity economy.CommodityID
	Quantity  int
	Buying    bool
}

func (StartHaggle) isCommand() {}

// HaggleOffer proposes Price (credits per unit) within an in-progress
// negotiation.
type HaggleOffer struct {
	Session haggle.Session
	Price   int
}

func (HaggleOffer) isCommand() {}

// HaggleWalkAway attempts to bluff a better price out of an in-progress
// negotiation by threatening to leave.
type HaggleWalkAway struct {
	Session haggle.Session
}

func (HaggleWalkAway) isCommand() {}

// HaggleAccept accepts the NPC's current offer, closing the negotiation and
// executing the trade.
type HaggleAccept struct {
	Session haggle.Session
}

func (HaggleAccept) isCommand() {}
