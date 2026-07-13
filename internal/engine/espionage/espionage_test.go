package espionage

import (
	"math/rand/v2"
	"testing"
)

func newRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
}

func TestStatusString(t *testing.T) {
	if StatusAvailable.String() != "available" {
		t.Fatalf("expected 'available', got %q", StatusAvailable.String())
	}
	if StatusCaptured.String() != "captured" {
		t.Fatalf("expected 'captured', got %q", StatusCaptured.String())
	}
}

func TestCounterIntelIncreasesWithDevelopmentLevel(t *testing.T) {
	prev := CounterIntel(1)
	for level := 2; level <= 5; level++ {
		ci := CounterIntel(level)
		if ci <= prev {
			t.Fatalf("expected CounterIntel to increase with development level, got %d at level %d after %d", ci, level, prev)
		}
		prev = ci
	}
}

func TestResolveIsDeterministic(t *testing.T) {
	for seed := uint64(0); seed < 100; seed++ {
		a := Resolve(newRand(seed), 50, 50)
		b := Resolve(newRand(seed), 50, 50)
		if a != b {
			t.Fatalf("seed %d: expected identical outcomes, got %+v and %+v", seed, a, b)
		}
	}
}

func TestResolveHighSkillSucceedsMoreOftenThanLowSkill(t *testing.T) {
	const trials = 5000
	var highWins, lowWins int
	for seed := uint64(0); seed < trials; seed++ {
		if Resolve(newRand(seed), 90, 50).Success {
			highWins++
		}
		if Resolve(newRand(seed+1_000_000), 10, 50).Success {
			lowWins++
		}
	}
	if highWins <= lowWins {
		t.Fatalf("expected high skill to succeed more often than low skill, got high=%d low=%d", highWins, lowWins)
	}
}

func TestResolveNeverCapturesOnSuccess(t *testing.T) {
	for seed := uint64(0); seed < 5000; seed++ {
		o := Resolve(newRand(seed), 95, 5)
		if o.Success && o.Captured {
			t.Fatalf("seed %d: expected a successful mission to never also be captured, got %+v", seed, o)
		}
	}
}

func TestResolveClampsExtremeChances(t *testing.T) {
	const trials = 3000
	var successes int
	for seed := uint64(0); seed < trials; seed++ {
		if Resolve(newRand(seed), 0, 1000).Success {
			successes++
		}
	}
	rate := float64(successes) / float64(trials)
	if rate < minSuccessChance-0.03 || rate > minSuccessChance+0.03 {
		t.Fatalf("expected success rate near the floor %.2f, got %.3f", minSuccessChance, rate)
	}
}
