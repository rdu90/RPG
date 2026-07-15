// Package companion provides the game's single narrative voice: a
// Jeeves-like companion (polite, deferential, dry) that flavors status
// messages shown to the player. It is presentation-only — plain functions
// returning strings, no engine or transport dependency.
//
// There is only one tone today. player.Alignment (Legality/Morality),
// already exposed on query.Player, is the intended future input for
// alignment-driven tone variants — a more roguish voice for a criminal
// player, a warmer one for a lawful trader — but wiring that up is
// deliberately out of scope for now: these functions take only the facts
// they need to phrase a message, not a tone parameter.
package companion

import "fmt"

// ColonyHint describes the option to found a colony at the player's current
// system, phrased differently depending on whether they can currently
// afford it.
func ColonyHint(canAfford bool, cost, turns, shortfall int) string {
	if canAfford {
		return fmt.Sprintf(
			"A colony could be founded here, sir, whenever you're ready — %d credits and %d turns ought to see it done. Press p at your convenience.",
			cost, turns)
	}
	return fmt.Sprintf(
		"A colony could be founded here for %d credits and %d turns, sir — though I note the treasury is presently short by %d credits. Press p once the funds allow.",
		cost, turns, shortfall)
}

// AlreadyInvestigated reports that the player has already investigated the
// anomaly of the given kind at their current system. kind is typically an
// explore.Kind, accepted here as a Stringer so this package doesn't need to
// import the engine.
func AlreadyInvestigated(kind fmt.Stringer) string {
	return fmt.Sprintf("You've already had a look at the %s here, sir — nothing more to find, I'm afraid.", kind)
}
