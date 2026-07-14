// Package combat resolves hostile encounters met while traveling: a
// deterministic, formula + seeded-RNG resolution over discrete rounds,
// presented as a result log — not a manual tactical-positioning engine. It
// has no dependency on persistence or transport — Hostile is a plain value
// the caller round-trips from the Move that generated it into
// ResolveEncounter, the same way haggle.Session is round-tripped across
// negotiation rounds, so the engine never needs to persist in-progress
// combat state.
package combat

import (
	"fmt"
	"math/rand/v2"

	"github.com/rdu90/RPG/internal/engine/fleet"
)

// Hostile is a single NPC raider encountered on arrival at a system,
// generated fresh from that system's development level. Seed is an opaque
// nonce identifying this specific encounter: echoing it back into a battle
// reproduces the exact same fight deterministically.
type Hostile struct {
	Seed    int64
	Name    string
	Attack  int
	Defense int
	Hull    int
	MaxHull int
}

var hostileNames = []string{
	"Raider", "Marauder", "Corsair", "Reaver", "Scavenger", "Renegade", "Buccaneer", "Privateer",
}

const (
	// EncounterBaseChance and encounterDevLevelStep set the odds of
	// meeting a hostile on arrival: lawless frontier systems (low
	// DevelopmentLevel) see less patrol traffic than developed core
	// worlds, so they're more dangerous, not less.
	EncounterBaseChance    = 0.30
	encounterDevLevelStep  = 0.05
	encounterChanceMinimum = 0.05

	hostileBaseAttack   = 8
	hostileAttackPerDev = 4
	hostileAttackJitter = 6

	hostileBaseDefense   = 4
	hostileDefensePerDev = 2
	hostileDefenseJitter = 4

	hostileBaseHull   = 30
	hostileHullPerDev = 10
	hostileHullJitter = 10
)

// EncounterChance returns the odds of meeting a hostile on arrival at a
// system of the given development level.
func EncounterChance(developmentLevel int) float64 {
	c := EncounterBaseChance - float64(developmentLevel-1)*encounterDevLevelStep
	if c < encounterChanceMinimum {
		c = encounterChanceMinimum
	}
	return c
}

// Generate rolls a fresh Hostile scaled to developmentLevel. The same seed
// and RNG state always produce the same Hostile.
func Generate(r *rand.Rand, seed int64, developmentLevel int) Hostile {
	maxHull := hostileBaseHull + developmentLevel*hostileHullPerDev + r.IntN(hostileHullJitter+1)
	return Hostile{
		Seed:    seed,
		Name:    hostileNames[r.IntN(len(hostileNames))],
		Attack:  hostileBaseAttack + developmentLevel*hostileAttackPerDev + r.IntN(hostileAttackJitter+1),
		Defense: hostileBaseDefense + developmentLevel*hostileDefensePerDev + r.IntN(hostileDefenseJitter+1),
		Hull:    maxHull,
		MaxHull: maxHull,
	}
}

// Stats returns h's combat capabilities as fleet.Stats, so battle
// resolution can treat both sides uniformly.
func (h Hostile) Stats() fleet.Stats {
	return fleet.Stats{Attack: h.Attack, Defense: h.Defense, Hull: h.Hull, MaxHull: h.MaxHull}
}

// Outcome is how a resolved encounter concluded.
type Outcome int

const (
	// Victory means the hostile was destroyed.
	Victory Outcome = iota
	// Defeat means the player's ship was disabled; it limps home rather
	// than being destroyed outright — there is no permadeath in v1.
	Defeat
	// Disengaged means neither side finished the other off within
	// maxRounds, and the hostile broke off the attack.
	Disengaged
)

// String renders the outcome for display/logging.
func (o Outcome) String() string {
	switch o {
	case Victory:
		return "victory"
	case Defeat:
		return "defeat"
	case Disengaged:
		return "disengaged"
	default:
		return "unknown"
	}
}

// maxRounds caps a fight's length so it always reaches a conclusion even
// between two heavily-armored, low-damage combatants.
const maxRounds = 10

const (
	// fleeBaseChance is the odds a flee attempt succeeds when the
	// player's Defense exactly matches the hostile's Attack; each point
	// of advantage shifts it by fleeChancePerPoint, clamped so fleeing is
	// never a certainty or an impossibility.
	fleeBaseChance     = 0.5
	fleeChancePerPoint = 0.01
	fleeChanceMinimum  = 0.15
	fleeChanceMaximum  = 0.85
)

// AttemptFlee resolves a flee attempt made before engaging: the player's
// Defense against the hostile's Attack decides the odds, the same shape as
// espionage.Resolve's single probability check.
func AttemptFlee(r *rand.Rand, player fleet.Stats, hostile Hostile) bool {
	chance := fleeBaseChance + float64(player.Defense-hostile.Attack)*fleeChancePerPoint
	if chance < fleeChanceMinimum {
		chance = fleeChanceMinimum
	} else if chance > fleeChanceMaximum {
		chance = fleeChanceMaximum
	}
	return r.Float64() < chance
}

// Result is a resolved battle: the terminal Outcome, both sides' final
// Hull, and a round-by-round log for display.
type Result struct {
	Outcome     Outcome
	PlayerHull  int
	HostileHull int
	Log         []string
}

// Fight resolves combat between player and hostile over discrete rounds:
// each round both sides land a hit in turn, continuing until one side's
// hull is exhausted or maxRounds passes.
func Fight(player fleet.Stats, hostile Hostile) Result {
	hs := hostile.Stats()
	var log []string

	for round := 1; round <= maxRounds; round++ {
		dmgToHostile := fleet.Damage(player.Attack, hs.Defense)
		hs = hs.TakeDamage(dmgToHostile)
		log = append(log, fmt.Sprintf("Round %d: you hit the %s for %d damage.", round, hostile.Name, dmgToHostile))
		if !hs.Alive() {
			log = append(log, fmt.Sprintf("The %s is destroyed!", hostile.Name))
			return Result{Outcome: Victory, PlayerHull: player.Hull, HostileHull: 0, Log: log}
		}

		dmgToPlayer := fleet.Damage(hs.Attack, player.Defense)
		player = player.TakeDamage(dmgToPlayer)
		log = append(log, fmt.Sprintf("Round %d: the %s hits you for %d damage.", round, hostile.Name, dmgToPlayer))
		if !player.Alive() {
			log = append(log, "Your ship is disabled — the raiders loot your hold and leave you adrift.")
			return Result{Outcome: Defeat, PlayerHull: 0, HostileHull: hs.Hull, Log: log}
		}
	}

	log = append(log, fmt.Sprintf("The %s breaks off the attack.", hostile.Name))
	return Result{Outcome: Disengaged, PlayerHull: player.Hull, HostileHull: hs.Hull, Log: log}
}
