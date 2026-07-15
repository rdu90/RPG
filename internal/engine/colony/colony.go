// Package colony models planetary colonies: a population that grows toward
// a development-level-derived cap on a coarse galaxy tick (separate from,
// and slower than, the player's turn allowance), and a production Focus
// commodity whose local market price drifts down as the colony's output
// grows. Like turn.Allowance, growth is computed lazily from elapsed real
// time rather than a running scheduler, so the same code resolves a colony
// whether it's backing a local save or a shared server.
package colony

import (
	"time"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/fleet"
	"github.com/rdu90/RPG/internal/engine/galaxy"
)

// Colony is a single system's planetary settlement.
type Colony struct {
	NodeID     galaxy.NodeID
	Focus      economy.CommodityID
	Population int
	LastTickAt time.Time

	// Owner is OwnerPlayer for a player-founded colony, or a
	// faction.Faction ID for a rival-held one. Garrison is meaningful only
	// when Owner is not OwnerPlayer: the defensive fleet a Bombard/Invade
	// action wears down or must defeat to capture the colony.
	Owner    string
	Garrison fleet.Stats
}

// OwnerPlayer is the sentinel Owner value for a colony founded by the
// player, as opposed to a rival faction.
const OwnerPlayer = "player"

const (
	// startingPopulation is a new colony's population at founding.
	startingPopulation = 50

	// tickInterval is how often a colony's population and production
	// advance, computed lazily from elapsed real time. Deliberately coarser
	// than the player's turn refill, matching the galaxy tick described in
	// PLAN.md.
	tickInterval = 60 * time.Second

	// growthDivisor controls how quickly population closes the gap to its
	// cap: each tick it moves 1/growthDivisor of the remaining distance,
	// floored at 1 so a colony never stalls before reaching its cap.
	growthDivisor = 15

	// basePopulationCap and capPerDevelopmentLevel derive a colony's
	// population ceiling from its system's development level: more
	// developed systems support larger colonies.
	basePopulationCap      = 1000
	capPerDevelopmentLevel = 400

	// priceDecayPerTick is the fraction a colony's Focus commodity price
	// drops by each tick, modeling growing local supply from production.
	priceDecayPerTick = 0.03

	// priceFloorFraction is the lowest a colony's production can push its
	// Focus commodity's price, as a fraction of that commodity's base price.
	priceFloorFraction = 0.3
)

// New founds a player-owned colony at nodeID producing focus, starting at
// now.
func New(nodeID galaxy.NodeID, focus economy.CommodityID, now time.Time) Colony {
	return Colony{NodeID: nodeID, Focus: focus, Population: startingPopulation, LastTickAt: now, Owner: OwnerPlayer}
}

// NewRival founds a rival-faction-owned colony at nodeID, used only when
// seeding the galaxy at game creation. Unlike a player-founded colony, its
// starting population and defensive garrison are supplied by the caller
// rather than starting from scratch.
func NewRival(nodeID galaxy.NodeID, focus economy.CommodityID, owner string, population int, garrison fleet.Stats, now time.Time) Colony {
	return Colony{NodeID: nodeID, Focus: focus, Population: population, LastTickAt: now, Owner: owner, Garrison: garrison}
}

// PopulationCap returns the population ceiling for a colony at a system of
// the given development level.
func PopulationCap(developmentLevel int) int {
	return basePopulationCap + capPerDevelopmentLevel*developmentLevel
}

// Ticked advances c to now, growing Population toward its development-level
// cap by one step per elapsed tickInterval. It returns the updated colony
// and how many ticks were applied, so the caller can also decay the Focus
// commodity's market price by the same number of steps.
func (c Colony) Ticked(now time.Time, developmentLevel int) (Colony, int) {
	if !now.After(c.LastTickAt) {
		return c, 0
	}
	ticks := int(now.Sub(c.LastTickAt) / tickInterval)
	if ticks <= 0 {
		return c, 0
	}

	cap := PopulationCap(developmentLevel)
	applied := 0
	for ; applied < ticks; applied++ {
		if c.Population >= cap {
			break
		}
		growth := (cap - c.Population) / growthDivisor
		if growth < 1 {
			growth = 1
		}
		c.Population += growth
		if c.Population > cap {
			c.Population = cap
		}
	}

	c.LastTickAt = c.LastTickAt.Add(time.Duration(ticks) * tickInterval)
	return c, ticks
}

// DecayedPrice applies ticks rounds of production-driven price decay to
// price (the Focus commodity's current market price at the colony's
// system), floored at priceFloorFraction of basePrice.
func DecayedPrice(price, basePrice, ticks int) int {
	floor := int(float64(basePrice) * priceFloorFraction)
	if floor < 1 {
		floor = 1
	}
	for i := 0; i < ticks; i++ {
		if price <= floor {
			break
		}
		drop := int(float64(price) * priceDecayPerTick)
		if drop < 1 {
			drop = 1
		}
		price -= drop
	}
	if price < floor {
		price = floor
	}
	return price
}
