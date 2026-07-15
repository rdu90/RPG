package combat

import (
	"math/rand/v2"
	"testing"

	"github.com/rdu90/RPG/internal/engine/fleet"
)

func newRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
}

func TestEncounterChanceDecreasesWithDevelopmentLevel(t *testing.T) {
	prev := EncounterChance(1)
	for level := 2; level <= 5; level++ {
		c := EncounterChance(level)
		if c > prev {
			t.Fatalf("expected EncounterChance to decrease with development level, got %.2f at level %d after %.2f", c, level, prev)
		}
		prev = c
	}
}

func TestEncounterChanceFloorsAtMinimum(t *testing.T) {
	if c := EncounterChance(100); c != encounterChanceMinimum {
		t.Fatalf("expected chance to floor at %.2f, got %.2f", encounterChanceMinimum, c)
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	for seed := uint64(0); seed < 50; seed++ {
		a := Generate(newRand(seed), int64(seed), 3)
		b := Generate(newRand(seed), int64(seed), 3)
		if a != b {
			t.Fatalf("seed %d: expected identical hostiles, got %+v and %+v", seed, a, b)
		}
	}
}

func TestGenerateScalesWithDevelopmentLevel(t *testing.T) {
	const trials = 500
	var lowHull, highHull int
	for seed := uint64(0); seed < trials; seed++ {
		lowHull += Generate(newRand(seed), int64(seed), 1).MaxHull
		highHull += Generate(newRand(seed+1_000_000), int64(seed), 5).MaxHull
	}
	if highHull <= lowHull {
		t.Fatalf("expected higher development level to produce tougher hostiles on average, got low=%d high=%d", lowHull, highHull)
	}
}

func TestAttemptFleeIsDeterministic(t *testing.T) {
	player := fleet.Stats{Attack: 10, Defense: 10, Hull: 50, MaxHull: 50}
	hostile := Hostile{Attack: 10, Defense: 5, Hull: 30, MaxHull: 30}
	for seed := uint64(0); seed < 100; seed++ {
		a := AttemptFlee(newRand(seed), player, hostile)
		b := AttemptFlee(newRand(seed), player, hostile)
		if a != b {
			t.Fatalf("seed %d: expected identical flee outcomes, got %v and %v", seed, a, b)
		}
	}
}

func TestAttemptFleeHigherDefenseSucceedsMoreOften(t *testing.T) {
	const trials = 5000
	hostile := Hostile{Attack: 30, Defense: 5, Hull: 30, MaxHull: 30}
	var strongWins, weakWins int
	for seed := uint64(0); seed < trials; seed++ {
		if AttemptFlee(newRand(seed), fleet.Stats{Defense: 60}, hostile) {
			strongWins++
		}
		if AttemptFlee(newRand(seed+1_000_000), fleet.Stats{Defense: 0}, hostile) {
			weakWins++
		}
	}
	if strongWins <= weakWins {
		t.Fatalf("expected higher defense to flee more often, got strong=%d weak=%d", strongWins, weakWins)
	}
}

func TestFightStrongPlayerWinsAgainstWeakHostile(t *testing.T) {
	player := fleet.Stats{Attack: 100, Defense: 100, Hull: 200, MaxHull: 200}
	hostile := Hostile{Name: "Raider", Attack: 5, Defense: 1, Hull: 10, MaxHull: 10}
	result := Fight(player, hostile)
	if result.Outcome != Victory {
		t.Fatalf("expected Victory, got %s", result.Outcome)
	}
	if result.HostileHull != 0 {
		t.Fatalf("expected hostile hull 0, got %d", result.HostileHull)
	}
	if len(result.Log) == 0 {
		t.Fatal("expected a non-empty battle log")
	}
}

func TestFightWeakPlayerLosesAgainstStrongHostile(t *testing.T) {
	player := fleet.Stats{Attack: 1, Defense: 1, Hull: 10, MaxHull: 10}
	hostile := Hostile{Name: "Marauder", Attack: 100, Defense: 100, Hull: 200, MaxHull: 200}
	result := Fight(player, hostile)
	if result.Outcome != Defeat {
		t.Fatalf("expected Defeat, got %s", result.Outcome)
	}
	if result.PlayerHull != 0 {
		t.Fatalf("expected player hull 0, got %d", result.PlayerHull)
	}
}

func TestFightEvenMatchupDisengagesWithinRoundCap(t *testing.T) {
	player := fleet.Stats{Attack: 1, Defense: 100, Hull: 200, MaxHull: 200}
	hostile := Hostile{Name: "Corsair", Attack: 1, Defense: 100, Hull: 200, MaxHull: 200}
	result := Fight(player, hostile)
	if result.Outcome != Disengaged {
		t.Fatalf("expected Disengaged, got %s", result.Outcome)
	}
	if len(result.Log) != maxRounds*2+1 {
		t.Fatalf("expected %d log lines (2 per round plus a closing line), got %d", maxRounds*2+1, len(result.Log))
	}
}

func TestFightIsDeterministic(t *testing.T) {
	player := fleet.Stats{Attack: 12, Defense: 6, Hull: 50, MaxHull: 50}
	hostile := Hostile{Name: "Reaver", Attack: 14, Defense: 5, Hull: 40, MaxHull: 40}
	a := Fight(player, hostile)
	b := Fight(player, hostile)
	if a.Outcome != b.Outcome || a.PlayerHull != b.PlayerHull || a.HostileHull != b.HostileHull {
		t.Fatalf("expected identical results from identical inputs, got %+v and %+v", a, b)
	}
}

func TestGenerateGarrisonIsDeterministic(t *testing.T) {
	for seed := uint64(0); seed < 50; seed++ {
		a := GenerateGarrison(newRand(seed), 3)
		b := GenerateGarrison(newRand(seed), 3)
		if a != b {
			t.Fatalf("seed %d: expected identical garrisons, got %+v and %+v", seed, a, b)
		}
	}
}

func TestGenerateGarrisonIsFortifiedAboveAnEquivalentHostile(t *testing.T) {
	const trials = 200
	var hostileHull, garrisonHull int
	for seed := uint64(0); seed < trials; seed++ {
		hostileHull += Generate(newRand(seed), int64(seed), 3).MaxHull
		garrisonHull += GenerateGarrison(newRand(seed), 3).MaxHull
	}
	if garrisonHull <= hostileHull {
		t.Fatalf("expected a garrison to be tougher on average than an equivalent wandering hostile, got hostile=%d garrison=%d", hostileHull, garrisonHull)
	}
}
