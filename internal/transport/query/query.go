// Package query re-exports the engine's Query vocabulary and result types
// under a transport-facing import path, for the same reason as
// internal/transport/command: callers above the transport boundary never
// import internal/engine directly.
package query

import (
	"github.com/rdu90/RPG/internal/engine"
	"github.com/rdu90/RPG/internal/engine/colony"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/explore"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/haggle"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
	"github.com/rdu90/RPG/internal/engine/techtree"
)

// GetGame returns the current save's identity record.
type GetGame = engine.GetGame

// GetGalaxy returns the save's full generated galaxy graph.
type GetGalaxy = engine.GetGalaxy

// GetPlayer returns the current player state.
type GetPlayer = engine.GetPlayer

// GetMarket returns commodity prices at the player's current system.
type GetMarket = engine.GetMarket

// GetAnomaly returns whether the player's current system hides an anomaly,
// and whether it has already been claimed.
type GetAnomaly = engine.GetAnomaly

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

// HaggleSession is the negotiation state carried between haggle rounds:
// StartHaggle returns one, and each subsequent Haggle* command takes and
// returns one, the same way a Command value round-trips.
type HaggleSession = haggle.Session

// HaggleResult is the result of StartHaggle and every subsequent Haggle*
// command.
type HaggleResult = engine.HaggleResult

// HaggleOutcome describes whether a negotiation is still in progress or how
// it concluded.
type HaggleOutcome = haggle.Outcome

// The possible values of HaggleOutcome.
const (
	HaggleInProgress = haggle.InProgress
	HaggleAccepted   = haggle.Accepted
	HaggleRejected   = haggle.Rejected
)

// ScoutResult is the result of a ScoutNode command: the newly-surveyed
// system's anomaly (if any), alongside the player's current state.
type ScoutResult = engine.ScoutResult

// ClaimAnomalyResult is the result of a ClaimAnomaly command: the anomaly
// just collected, alongside the player's current state.
type ClaimAnomalyResult = engine.ClaimAnomalyResult

// AnomalyStatus is the result of GetAnomaly: what (if anything) is hidden
// at a system, and whether it's already been claimed.
type AnomalyStatus = engine.AnomalyStatus

// GetColony returns the colony (if any) at the player's current system.
type GetColony = engine.GetColony

// GetColonies returns every colony in the save.
type GetColonies = engine.GetColonies

// ColonizeResult is the result of a Colonize command: the newly-founded
// colony, alongside the player's current state.
type ColonizeResult = engine.ColonizeResult

// ColonyStatus is the result of GetColony: whether a colony exists at the
// player's current system, and its state if so.
type ColonyStatus = engine.ColonyStatus

// Colony is a planetary settlement founded by Colonize: a Focus commodity
// it produces and a population that grows on the galaxy tick, feeding back
// into that commodity's local market price.
type Colony = colony.Colony

// ColonyPopulationCap returns a colony's population ceiling at a system of
// the given development level.
func ColonyPopulationCap(developmentLevel int) int { return colony.PopulationCap(developmentLevel) }

// ColonizeCost returns the credit cost of founding a colony at a system of
// the given development level.
func ColonizeCost(developmentLevel int) int { return engine.ColonizeCost(developmentLevel) }

// ColonizeTurnCost is the turn price of founding a colony, independent of
// its credit cost.
const ColonizeTurnCost = engine.ColonizeTurnCost

// GetTechTree returns the fixed tech catalog alongside the player's current
// research progress.
type GetTechTree = engine.GetTechTree

// StartResearchResult is the result of a StartResearch command: the
// player's research state after starting (or switching to) a project,
// alongside the player's current state.
type StartResearchResult = engine.StartResearchResult

// TechTreeStatus is the result of GetTechTree: the fixed tech catalog
// alongside the player's current research progress and state.
type TechTreeStatus = engine.TechTreeStatus

// Tech, TechID, EffectKind, and Research describe the tech tree returned by
// GetTechTree: a fixed catalog of research nodes and the player's progress
// through them.
type (
	Tech       = techtree.Tech
	TechID     = techtree.TechID
	EffectKind = techtree.EffectKind
	Research   = techtree.Research
)

// The possible values of EffectKind.
const (
	EffectCargoCapacity       = techtree.EffectCargoCapacity
	EffectTurnMax             = techtree.EffectTurnMax
	EffectTradeGreedReduction = techtree.EffectTradeGreedReduction
	EffectResearchRate        = techtree.EffectResearchRate
)

// FindTech looks up a tech definition by ID.
func FindTech(id TechID) (Tech, bool) { return techtree.Find(id) }

// Anomaly is a secret a system may hide, discoverable by scouting it or
// flying there.
type Anomaly = explore.Anomaly

// AnomalyKind identifies what sort of anomaly a system hides.
type AnomalyKind = explore.Kind

// The possible values of AnomalyKind.
const (
	AnomalyNone     = explore.KindNone
	AnomalyDerelict = explore.KindDerelict
	AnomalyBeacon   = explore.KindBeacon
	AnomalyCache    = explore.KindCache
)
