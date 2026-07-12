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
	Credits          int
	NodeID           galaxy.NodeID
	CargoCapacity    int
	Cargo            map[economy.CommodityID]int
	Turns            turn.Allowance
	Reputation       map[galaxy.NodeID]int
	Alignment        Alignment
	Discovered       map[galaxy.NodeID]bool
	ClaimedAnomalies map[galaxy.NodeID]bool
}

// CargoUsed returns the total units currently held across all commodities.
func (p Player) CargoUsed() int {
	used := 0
	for _, qty := range p.Cargo {
		used += qty
	}
	return used
}

// ReputationAt returns the player's standing at node, defaulting to 0 for a
// system never visited before.
func (p Player) ReputationAt(node galaxy.NodeID) int {
	return p.Reputation[node]
}

// HasDiscovered reports whether node has been surveyed, either by flying
// there or by scouting it from an adjacent system.
func (p Player) HasDiscovered(node galaxy.NodeID) bool {
	return p.Discovered[node]
}

// HasClaimedAnomaly reports whether the anomaly (if any) hidden at node has
// already been collected.
func (p Player) HasClaimedAnomaly(node galaxy.NodeID) bool {
	return p.ClaimedAnomalies[node]
}

// Alignment is the player's derived legal/moral standing, a 2D vector
// nudged toward each completed trade's commodity category rather than a
// directly-set field: what you trade is your alignment.
type Alignment struct {
	Legality float64 // -1 (criminal) .. +1 (lawful)
	Morality float64 // -1 (immoral) .. +1 (moral)
}

// alignmentSmoothing controls how much a single trade moves the running
// average: low enough that alignment reflects a sustained pattern of
// trading, not a single commodity switch.
const alignmentSmoothing = 0.15

// alignmentContribution is the axis pull of trading each commodity
// category once.
var alignmentContribution = map[economy.Category]Alignment{
	economy.CategoryNormal:  {Legality: 1, Morality: 1},
	economy.CategoryIllegal: {Legality: -1, Morality: 0},
	economy.CategoryExotic:  {Legality: 1, Morality: 0},
	economy.CategoryImmoral: {Legality: 0, Morality: -1},
}

// ContributionFor returns the alignment axis a trade in the given category
// pulls toward.
func ContributionFor(cat economy.Category) Alignment {
	return alignmentContribution[cat]
}

// Nudge moves a toward contribution by alignmentSmoothing, the exponential
// moving average that makes Alignment a weighted average over the player's
// trade ledger without needing to store the ledger itself.
func (a Alignment) Nudge(contribution Alignment) Alignment {
	return Alignment{
		Legality: a.Legality + (contribution.Legality-a.Legality)*alignmentSmoothing,
		Morality: a.Morality + (contribution.Morality-a.Morality)*alignmentSmoothing,
	}
}
