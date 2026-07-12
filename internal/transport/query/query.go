// Package query re-exports the engine's Query vocabulary and result types
// under a transport-facing import path, for the same reason as
// internal/transport/command: callers above the transport boundary never
// import internal/engine directly.
package query

import (
	"github.com/rdu90/RPG/internal/engine"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
)

// GetGame returns the current save's identity record.
type GetGame = engine.GetGame

// GetGalaxy returns the save's full generated galaxy graph.
type GetGalaxy = engine.GetGalaxy

// GetPlayer returns the current player state.
type GetPlayer = engine.GetPlayer

// GetMarket returns commodity prices at the player's current system.
type GetMarket = engine.GetMarket

// Game is the result type returned for GetGame and for a successful
// CreateGame command.
type Game = ports.Game

// Galaxy, Node, Edge, and NodeID are the result types for GetGalaxy.
type (
	Galaxy = galaxy.Galaxy
	Node   = galaxy.Node
	Edge   = galaxy.Edge
	NodeID = galaxy.NodeID
)

// Player is the result type for GetPlayer and for Move/Buy/Sell commands.
type Player = player.Player

// Commodity, CommodityID, Category, and Price describe the tradeable
// goods returned by GetMarket.
type (
	Commodity   = economy.Commodity
	CommodityID = economy.CommodityID
	Category    = economy.Category
	Price       = economy.Price
)

// Commodities is the fixed catalog of tradeable goods.
var Commodities = economy.Commodities

// FindCommodity looks up a commodity definition by ID.
func FindCommodity(id CommodityID) (Commodity, bool) { return economy.Find(id) }
