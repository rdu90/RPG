package engine

import (
	"github.com/rdu90/RPG/internal/engine/combat"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/espionage"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/haggle"
	"github.com/rdu90/RPG/internal/engine/techtree"
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

// ScoutNode surveys a system adjacent to the player's current one without
// flying there, revealing it (and any anomaly hidden there) at half the
// turn cost of a full flight.
type ScoutNode struct {
	To galaxy.NodeID
}

func (ScoutNode) isCommand() {}

// ClaimAnomaly collects the reward from an unclaimed anomaly at the
// player's current system.
type ClaimAnomaly struct{}

func (ClaimAnomaly) isCommand() {}

// Colonize founds a colony at the player's current system, producing Focus.
type Colonize struct {
	Focus economy.CommodityID
}

func (Colonize) isCommand() {}

// StartResearch begins researching Tech, replacing any in-progress project
// (its accumulated progress is lost).
type StartResearch struct {
	Tech techtree.TechID
}

func (StartResearch) isCommand() {}

// RecruitSpy hires a new spy for a flat credit and turn cost.
type RecruitSpy struct{}

func (RecruitSpy) isCommand() {}

// SendSpyMission sends Spy on a Mission against Target, resolving it
// immediately as a single probability check.
type SendSpyMission struct {
	Spy     string
	Target  galaxy.NodeID
	Mission espionage.MissionKind
}

func (SendSpyMission) isCommand() {}

// ResolveEncounter resolves a hostile encountered on arrival: either an
// attempted flee or a fight to a conclusion if fleeing isn't chosen (or
// fails). Hostile must be the exact value returned by the Move that
// generated it.
type ResolveEncounter struct {
	Hostile combat.Hostile
	Flee    bool
}

func (ResolveEncounter) isCommand() {}

// RepairShip restores the player's ship to full hull for a credit cost
// proportional to the damage repaired.
type RepairShip struct{}

func (RepairShip) isCommand() {}

// Bombard strikes the rival-faction colony at the player's current system
// from orbit, weakening its garrison and population without risking the
// player's ship.
type Bombard struct{}

func (Bombard) isCommand() {}

// Invade attempts to capture the rival-faction colony at the player's
// current system by fighting its garrison to a conclusion.
type Invade struct{}

func (Invade) isCommand() {}
