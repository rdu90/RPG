// Package command re-exports the engine's Command vocabulary under a
// transport-facing import path. Callers above the transport boundary (the
// TUI, and later a network client) depend on this package instead of
// internal/engine directly, so the engine package can be swapped for a
// remote implementation without touching caller code.
package command

import "github.com/rdu90/RPG/internal/engine"

// CreateGame starts a new save with the given name.
type CreateGame = engine.CreateGame

// Move flies the player's ship to an adjacent system.
type Move = engine.Move

// StartHaggle opens a negotiation over cargo at the player's current system.
type StartHaggle = engine.StartHaggle

// HaggleOffer proposes a price per unit within an in-progress negotiation.
type HaggleOffer = engine.HaggleOffer

// HaggleWalkAway attempts to bluff a better price out of an in-progress
// negotiation by threatening to leave.
type HaggleWalkAway = engine.HaggleWalkAway

// HaggleAccept accepts the NPC's current offer, closing the negotiation.
type HaggleAccept = engine.HaggleAccept

// ScoutNode surveys a system adjacent to the player's current one without
// flying there.
type ScoutNode = engine.ScoutNode

// ClaimAnomaly collects the reward from an unclaimed anomaly at the
// player's current system.
type ClaimAnomaly = engine.ClaimAnomaly

// Colonize founds a colony at the player's current system, producing Focus.
type Colonize = engine.Colonize

// StartResearch begins researching a tech, replacing any in-progress
// project (its accumulated progress is lost).
type StartResearch = engine.StartResearch
