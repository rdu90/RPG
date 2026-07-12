package explore

import (
	"math/rand/v2"
	"testing"
)

func newRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
}

func TestAtIsDeterministic(t *testing.T) {
	for seed := uint64(0); seed < 50; seed++ {
		a := At(newRand(seed), 3)
		b := At(newRand(seed), 3)
		if a != b {
			t.Fatalf("seed %d: expected identical rolls, got %+v and %+v", seed, a, b)
		}
	}
}

func TestAtRewardsWithinBounds(t *testing.T) {
	for seed := uint64(0); seed < 5000; seed++ {
		a := At(newRand(seed), 1+int(seed%5))
		switch a.Kind {
		case KindNone:
			if a.CreditsReward != 0 || a.ReputationReward != 0 {
				t.Fatalf("seed %d: expected zero rewards for KindNone, got %+v", seed, a)
			}
		case KindDerelict:
			if a.CreditsReward < derelictCreditsMin || a.CreditsReward > derelictCreditsMax {
				t.Fatalf("seed %d: derelict credits out of bounds: %+v", seed, a)
			}
			if a.ReputationReward != 0 {
				t.Fatalf("seed %d: expected derelict to have no reputation reward, got %+v", seed, a)
			}
		case KindBeacon:
			if a.ReputationReward < beaconRepMin || a.ReputationReward > beaconRepMax {
				t.Fatalf("seed %d: beacon reputation out of bounds: %+v", seed, a)
			}
			if a.CreditsReward != 0 {
				t.Fatalf("seed %d: expected beacon to have no credits reward, got %+v", seed, a)
			}
		case KindCache:
			if a.CreditsReward < cacheCreditsMin || a.CreditsReward > cacheCreditsMax {
				t.Fatalf("seed %d: cache credits out of bounds: %+v", seed, a)
			}
			if a.ReputationReward != 0 {
				t.Fatalf("seed %d: expected cache to have no reputation reward, got %+v", seed, a)
			}
		default:
			t.Fatalf("seed %d: unexpected kind %v", seed, a.Kind)
		}
	}
}

func TestAtFrontierIsMoreLikelyThanCore(t *testing.T) {
	const trials = 20000
	var frontierHits, coreHits int
	for seed := uint64(0); seed < trials; seed++ {
		if !At(newRand(seed), 1).Empty() {
			frontierHits++
		}
		if !At(newRand(seed+1_000_000), 5).Empty() {
			coreHits++
		}
	}
	if frontierHits <= coreHits {
		t.Fatalf("expected frontier systems to hide anomalies more often than core worlds, got frontier=%d core=%d", frontierHits, coreHits)
	}
}

func TestEmpty(t *testing.T) {
	if !(Anomaly{}).Empty() {
		t.Fatal("expected zero-value Anomaly to be Empty")
	}
	if (Anomaly{Kind: KindDerelict}).Empty() {
		t.Fatal("expected a KindDerelict anomaly to not be Empty")
	}
}
