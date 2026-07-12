package colony

import (
	"testing"
	"time"
)

func TestNewStartsBelowCap(t *testing.T) {
	now := time.Now().UTC()
	c := New("sys-000", "food", now)
	if c.Population != startingPopulation {
		t.Fatalf("expected starting population %d, got %d", startingPopulation, c.Population)
	}
	if c.Population >= PopulationCap(1) {
		t.Fatalf("expected starting population below cap, got %d >= %d", c.Population, PopulationCap(1))
	}
	if !c.LastTickAt.Equal(now) {
		t.Fatalf("expected LastTickAt %v, got %v", now, c.LastTickAt)
	}
}

func TestPopulationCapGrowsWithDevelopmentLevel(t *testing.T) {
	prev := PopulationCap(1)
	for level := 2; level <= 5; level++ {
		cap := PopulationCap(level)
		if cap <= prev {
			t.Fatalf("expected cap to increase with development level: level %d cap %d <= level %d cap %d",
				level, cap, level-1, prev)
		}
		prev = cap
	}
}

func TestTickedNoElapsedTimeIsNoOp(t *testing.T) {
	now := time.Now().UTC()
	c := New("sys-000", "food", now)

	updated, ticks := c.Ticked(now, 3)
	if ticks != 0 {
		t.Fatalf("expected 0 ticks with no elapsed time, got %d", ticks)
	}
	if updated != c {
		t.Fatalf("expected colony unchanged, got %+v", updated)
	}

	updated, ticks = c.Ticked(now.Add(-time.Hour), 3)
	if ticks != 0 || updated != c {
		t.Fatalf("expected no-op when now is before LastTickAt, got %+v ticks=%d", updated, ticks)
	}
}

func TestTickedGrowsPopulationTowardCap(t *testing.T) {
	now := time.Now().UTC()
	c := New("sys-000", "food", now)

	later := now.Add(5 * tickInterval)
	updated, ticks := c.Ticked(later, 3)
	if ticks != 5 {
		t.Fatalf("expected 5 ticks, got %d", ticks)
	}
	if updated.Population <= c.Population {
		t.Fatalf("expected population to grow, got %d <= %d", updated.Population, c.Population)
	}
	if updated.Population > PopulationCap(3) {
		t.Fatalf("expected population capped at %d, got %d", PopulationCap(3), updated.Population)
	}
	if !updated.LastTickAt.Equal(later) {
		t.Fatalf("expected LastTickAt advanced to %v, got %v", later, updated.LastTickAt)
	}
}

func TestTickedStopsGrowingAtCap(t *testing.T) {
	now := time.Now().UTC()
	c := New("sys-000", "food", now)

	// Enough elapsed ticks to fully converge on the cap.
	later := now.Add(10000 * tickInterval)
	updated, ticks := c.Ticked(later, 1)
	if ticks != 10000 {
		t.Fatalf("expected 10000 ticks reported even though growth converges early, got %d", ticks)
	}
	if updated.Population != PopulationCap(1) {
		t.Fatalf("expected population to reach cap %d, got %d", PopulationCap(1), updated.Population)
	}
}

func TestTickedIsDeterministic(t *testing.T) {
	now := time.Now().UTC()
	c := New("sys-000", "food", now)
	later := now.Add(7 * tickInterval)

	a, ticksA := c.Ticked(later, 4)
	b, ticksB := c.Ticked(later, 4)
	if a != b || ticksA != ticksB {
		t.Fatalf("expected deterministic result, got %+v/%d vs %+v/%d", a, ticksA, b, ticksB)
	}
}

func TestDecayedPriceDecreasesTowardFloor(t *testing.T) {
	price := DecayedPrice(100, 100, 0)
	if price != 100 {
		t.Fatalf("expected no decay with 0 ticks, got %d", price)
	}

	price = DecayedPrice(100, 100, 1)
	if price >= 100 {
		t.Fatalf("expected price to decrease after 1 tick, got %d", price)
	}

	price = DecayedPrice(100, 100, 10000)
	floor := int(float64(100) * priceFloorFraction)
	if price != floor {
		t.Fatalf("expected price to converge to floor %d, got %d", floor, price)
	}
}

func TestDecayedPriceNeverBelowFloorForLowBasePrice(t *testing.T) {
	price := DecayedPrice(3, 3, 10000)
	if price < 1 {
		t.Fatalf("expected price floored at least at 1, got %d", price)
	}
}
