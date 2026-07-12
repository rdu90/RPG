// Package explore models the anomaly "secrets" scouting and travel can
// uncover: a per-system roll of whether something valuable is hidden there,
// and if so what. It has no dependency on persistence or transport — At is a
// pure function of an RNG and a system's development level, the same shape
// as haggle.Start, so the engine layer supplies an already-seeded *rand.Rand
// derived from the save's ID and never persists any RNG state.
package explore

import "math/rand/v2"

// Kind identifies what sort of anomaly a system hides.
type Kind int

const (
	// KindNone means the system has nothing hidden at it.
	KindNone Kind = iota
	// KindDerelict is a wrecked ship or station: a modest one-off credit
	// reward.
	KindDerelict
	// KindBeacon is a distress beacon: rescuing/reporting it earns
	// standing with the local system rather than credits.
	KindBeacon
	// KindCache is a rare, well-hidden stash: a large credit reward.
	KindCache
)

// String names a Kind for display.
func (k Kind) String() string {
	switch k {
	case KindDerelict:
		return "derelict wreck"
	case KindBeacon:
		return "distress beacon"
	case KindCache:
		return "hidden cache"
	default:
		return "nothing"
	}
}

// Anomaly is what a system hides, if anything. A zero-value Anomaly (Kind ==
// KindNone) means nothing is hidden there.
type Anomaly struct {
	Kind             Kind
	CreditsReward    int
	ReputationReward int
}

// Empty reports whether there is nothing hidden at the system.
func (a Anomaly) Empty() bool { return a.Kind == KindNone }

const (
	// baseChance is the odds a core world (developmentLevel 5) hides an
	// anomaly; chancePerLevel adds to that for every level below 5, so
	// frontier systems (developmentLevel 1) are the most rewarding to
	// explore — core worlds are picked clean, the frontier isn't.
	baseChance      = 0.15
	chancePerLevel  = 0.05
	derelictRollMax = 0.55 // [0, 0.55) -> derelict
	beaconRollMax   = 0.85 // [0.55, 0.85) -> beacon; [0.85, 1) -> cache

	derelictCreditsMin = 50
	derelictCreditsMax = 200
	beaconRepMin       = 5
	beaconRepMax       = 15
	cacheCreditsMin    = 300
	cacheCreditsMax    = 800
)

// At deterministically rolls whether a system of the given development
// level hides an anomaly and, if so, what kind and reward. Calling it twice
// with fresh *rand.Rand values seeded identically always returns the same
// Anomaly, so the engine can call it repeatedly (on scout, on claim, on
// status query) without persisting the roll's outcome anywhere.
func At(r *rand.Rand, developmentLevel int) Anomaly {
	chance := baseChance + float64(5-developmentLevel)*chancePerLevel
	if r.Float64() > chance {
		return Anomaly{}
	}

	roll := r.Float64()
	switch {
	case roll < derelictRollMax:
		return Anomaly{
			Kind:          KindDerelict,
			CreditsReward: derelictCreditsMin + r.IntN(derelictCreditsMax-derelictCreditsMin+1),
		}
	case roll < beaconRollMax:
		return Anomaly{
			Kind:             KindBeacon,
			ReputationReward: beaconRepMin + r.IntN(beaconRepMax-beaconRepMin+1),
		}
	default:
		return Anomaly{
			Kind:          KindCache,
			CreditsReward: cacheCreditsMin + r.IntN(cacheCreditsMax-cacheCreditsMin+1),
		}
	}
}
