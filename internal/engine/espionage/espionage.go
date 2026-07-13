// Package espionage models recruited spies and the narrow set of missions
// they can run against a star system: a single probability check (spy
// Skill against the target system's CounterIntel), not a detection
// minigame or a persistent spy-network simulation. It has no dependency on
// persistence or transport — Resolve is a pure function of an RNG and the
// two stats, the same shape as haggle.Start and explore.At, so the engine
// layer supplies an already-seeded *rand.Rand and never persists any RNG
// state.
package espionage

import "math/rand/v2"

// Status is a spy's availability.
type Status int

const (
	// StatusAvailable means the spy can be sent on a mission.
	StatusAvailable Status = iota
	// StatusCaptured means the spy was lost on a failed mission and can no
	// longer be sent on missions.
	StatusCaptured
)

// String names a Status for display.
func (s Status) String() string {
	if s == StatusCaptured {
		return "captured"
	}
	return "available"
}

// Spy is a recruited operative the player can send on missions. Skill never
// changes after recruitment — there is no leveling or training mechanic in
// v1.
type Spy struct {
	ID          string
	Name        string
	Skill       int // 0-100
	Status      Status
	MissionsRun int
}

// MissionKind identifies the narrow set of missions a spy can run.
type MissionKind string

const (
	// MissionSteal robs the target system's coffers for a credit reward.
	MissionSteal MissionKind = "steal"
	// MissionSabotage disrupts the target system's economy, crashing its
	// market prices.
	MissionSabotage MissionKind = "sabotage"
	// MissionIntel remotely surveys the target system without traveling
	// there, the same reveal a visit or a scout produces.
	MissionIntel MissionKind = "intel"
)

// CounterIntel derives a target system's counter-espionage strength from
// its development level: better-defended core worlds are harder to work
// against than frontier outposts.
func CounterIntel(developmentLevel int) int {
	return 20 + developmentLevel*12 // 32..80
}

const (
	// baseSuccessChance is the odds of success when spy Skill exactly
	// matches the target's CounterIntel; each point of advantage shifts it
	// by skillChancePerPoint, clamped to [minSuccessChance, maxSuccessChance]
	// so a mission is never a certainty or an impossibility.
	baseSuccessChance   = 0.5
	skillChancePerPoint = 0.01
	minSuccessChance    = 0.05
	maxSuccessChance    = 0.95
	captureChanceOnFail = 0.4
)

// Outcome is the result of resolving a single mission attempt.
type Outcome struct {
	Success  bool
	Captured bool
}

// Resolve runs the single probability check a mission attempt boils down
// to: skill against counterIntel decides success; a failed mission then has
// a further chance of costing the spy their freedom.
func Resolve(r *rand.Rand, skill, counterIntel int) Outcome {
	chance := baseSuccessChance + float64(skill-counterIntel)*skillChancePerPoint
	if chance < minSuccessChance {
		chance = minSuccessChance
	} else if chance > maxSuccessChance {
		chance = maxSuccessChance
	}

	if r.Float64() < chance {
		return Outcome{Success: true}
	}
	return Outcome{Captured: r.Float64() < captureChanceOnFail}
}
