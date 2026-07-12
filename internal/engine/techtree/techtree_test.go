package techtree

import (
	"strconv"
	"testing"
	"time"
)

func TestCatalogShape(t *testing.T) {
	if len(Catalog) != len(branches)*tiersPerBranch {
		t.Fatalf("expected %d techs, got %d", len(branches)*tiersPerBranch, len(Catalog))
	}

	seen := map[TechID]bool{}
	for _, b := range branches {
		var prev TechID
		for tier := 1; tier <= tiersPerBranch; tier++ {
			id := TechID(b.prefix + "-" + strconv.Itoa(tier))
			tech, ok := Find(id)
			if !ok {
				t.Fatalf("expected %s in catalog", id)
			}
			if seen[id] {
				t.Fatalf("duplicate tech id %s", id)
			}
			seen[id] = true

			if tech.Tier != tier {
				t.Fatalf("%s: expected tier %d, got %d", id, tier, tech.Tier)
			}
			if tech.Cost != costPerTier*tier {
				t.Fatalf("%s: expected cost %d, got %d", id, costPerTier*tier, tech.Cost)
			}
			if tech.Prerequisite != prev {
				t.Fatalf("%s: expected prerequisite %q, got %q", id, prev, tech.Prerequisite)
			}
			if tech.Effect.Kind != b.kind {
				t.Fatalf("%s: expected effect kind %s, got %s", id, b.kind, tech.Effect.Kind)
			}
			if tech.Effect.Magnitude != b.magnitudePerTier*tier {
				t.Fatalf("%s: expected magnitude %d, got %d", id, b.magnitudePerTier*tier, tech.Effect.Magnitude)
			}
			prev = id
		}
	}
}

func TestFindUnknownTech(t *testing.T) {
	if _, ok := Find("does-not-exist"); ok {
		t.Fatal("expected unknown tech to not be found")
	}
}

func TestAvailableRequiresPrerequisite(t *testing.T) {
	r := Research{}
	if r.Available("cargo-2") {
		t.Fatal("expected tier 2 to be unavailable before tier 1 is unlocked")
	}
	if !r.Available("cargo-1") {
		t.Fatal("expected tier 1 (no prerequisite) to be available")
	}

	r.Unlocked = map[TechID]bool{"cargo-1": true}
	if !r.Available("cargo-2") {
		t.Fatal("expected tier 2 to become available once tier 1 is unlocked")
	}
}

func TestAvailableFalseOnceUnlocked(t *testing.T) {
	r := Research{Unlocked: map[TechID]bool{"cargo-1": true}}
	if r.Available("cargo-1") {
		t.Fatal("expected an already-unlocked tech to not be available")
	}
}

func TestAvailableFalseForUnknownTech(t *testing.T) {
	r := Research{}
	if r.Available("bogus") {
		t.Fatal("expected an unknown tech id to not be available")
	}
}

func TestStartRejectsUnavailable(t *testing.T) {
	r := Research{}
	if _, err := r.Start("cargo-2", time.Now()); err != ErrNotResearchable {
		t.Fatalf("expected ErrNotResearchable, got %v", err)
	}
}

func TestStartSetsActiveAndResetsProgress(t *testing.T) {
	now := time.Now().UTC()
	r := Research{Progress: 999, Active: "trade-1"}
	r, err := r.Start("cargo-1", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Active != "cargo-1" {
		t.Fatalf("expected active cargo-1, got %s", r.Active)
	}
	if r.Progress != 0 {
		t.Fatalf("expected progress reset to 0, got %d", r.Progress)
	}
	if !r.LastTickAt.Equal(now) {
		t.Fatalf("expected LastTickAt %v, got %v", now, r.LastTickAt)
	}
}

func TestTickedNoActiveIsNoOp(t *testing.T) {
	now := time.Now().UTC()
	r := Research{LastTickAt: now.Add(-time.Hour)}
	updated, ticks, completed := r.Ticked(now)
	if ticks != 0 || completed != "" {
		t.Fatalf("expected no-op with no active project, got ticks=%d completed=%s", ticks, completed)
	}
	if updated.Progress != 0 {
		t.Fatalf("expected unchanged progress, got %d", updated.Progress)
	}
}

func TestTickedNoElapsedTimeIsNoOp(t *testing.T) {
	now := time.Now().UTC()
	r := Research{Active: "cargo-1", LastTickAt: now}
	_, ticks, completed := r.Ticked(now)
	if ticks != 0 || completed != "" {
		t.Fatalf("expected no-op with no elapsed time, got ticks=%d completed=%s", ticks, completed)
	}
}

func TestTickedAccruesProgressWithoutCompleting(t *testing.T) {
	start := time.Now().UTC()
	r := Research{Active: "cargo-1", LastTickAt: start} // cost 40, base rate 4/tick
	now := start.Add(2 * tickInterval)                  // 2 ticks * 4 = 8 progress, well under 40

	updated, ticks, completed := r.Ticked(now)
	if ticks != 2 {
		t.Fatalf("expected 2 ticks, got %d", ticks)
	}
	if completed != "" {
		t.Fatalf("expected no completion yet, got %s", completed)
	}
	if updated.Progress != 8 {
		t.Fatalf("expected progress 8, got %d", updated.Progress)
	}
	if updated.Active != "cargo-1" {
		t.Fatalf("expected still researching cargo-1, got %s", updated.Active)
	}
}

func TestTickedCompletesAndUnlocksResearchRateEffect(t *testing.T) {
	start := time.Now().UTC()
	r := Research{Active: "research-1", LastTickAt: start} // cost 40, magnitude 1, base rate 4/tick
	now := start.Add(10 * tickInterval)                    // 10 * 4 = 40, exactly the cost

	updated, ticks, completed := r.Ticked(now)
	if ticks != 10 {
		t.Fatalf("expected 10 ticks, got %d", ticks)
	}
	if completed != "research-1" {
		t.Fatalf("expected research-1 to complete, got %q", completed)
	}
	if !updated.HasUnlocked("research-1") {
		t.Fatal("expected research-1 to be unlocked")
	}
	if updated.RateBonus != 1 {
		t.Fatalf("expected RateBonus 1, got %d", updated.RateBonus)
	}
	if updated.Active != "" {
		t.Fatalf("expected Active cleared after completion, got %s", updated.Active)
	}
	if updated.Progress != 0 {
		t.Fatalf("expected progress reset after completion, got %d", updated.Progress)
	}
	if updated.RatePerTick() != baseRatePerTick+1 {
		t.Fatalf("expected boosted rate %d, got %d", baseRatePerTick+1, updated.RatePerTick())
	}
}

func TestTickedCompletesAndAppliesTradeGreedReduction(t *testing.T) {
	start := time.Now().UTC()
	r := Research{Active: "trade-1", LastTickAt: start} // cost 40, magnitude 2
	now := start.Add(10 * tickInterval)

	updated, _, completed := r.Ticked(now)
	if completed != "trade-1" {
		t.Fatalf("expected trade-1 to complete, got %q", completed)
	}
	if updated.TradeGreedReduction != 2 {
		t.Fatalf("expected TradeGreedReduction 2, got %d", updated.TradeGreedReduction)
	}
}

func TestTickedCompletesAtMostOneTechPerCall(t *testing.T) {
	start := time.Now().UTC()
	r := Research{Active: "cargo-1", LastTickAt: start} // cost 40
	// A huge gap that would, at base rate, accrue far more than one tier's
	// cost: completion should still only unlock cargo-1, not cascade into
	// starting or completing cargo-2 automatically.
	now := start.Add(1000 * tickInterval)

	updated, _, completed := r.Ticked(now)
	if completed != "cargo-1" {
		t.Fatalf("expected cargo-1 to complete, got %q", completed)
	}
	if updated.HasUnlocked("cargo-2") {
		t.Fatal("expected cargo-2 to not be auto-unlocked")
	}
	if updated.Active != "" {
		t.Fatalf("expected no project auto-started, got %s", updated.Active)
	}
}

func TestTickedIsDeterministic(t *testing.T) {
	start := time.Now().UTC()
	r := Research{Active: "cargo-1", LastTickAt: start}
	now := start.Add(3 * tickInterval)

	a, aTicks, aCompleted := r.Ticked(now)
	b, bTicks, bCompleted := r.Ticked(now)
	if aTicks != bTicks || aCompleted != bCompleted || a.Progress != b.Progress {
		t.Fatalf("expected deterministic results, got (%d,%v,%d) vs (%d,%v,%d)",
			aTicks, aCompleted, a.Progress, bTicks, bCompleted, b.Progress)
	}
}
