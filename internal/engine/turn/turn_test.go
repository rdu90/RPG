package turn

import (
	"errors"
	"testing"
	"time"
)

func TestNewIsFull(t *testing.T) {
	now := time.Now()
	a := New(60, time.Minute, now)
	if a.Remaining != 60 || a.Max != 60 {
		t.Fatalf("expected full allowance, got %+v", a)
	}
}

func TestSpendReducesRemaining(t *testing.T) {
	now := time.Now()
	a := New(60, time.Minute, now)

	a, err := a.Spend(now, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Remaining != 50 {
		t.Fatalf("expected 50 remaining, got %d", a.Remaining)
	}
}

func TestSpendInsufficientTurns(t *testing.T) {
	now := time.Now()
	a := New(5, time.Minute, now)

	_, err := a.Spend(now, 10)
	if !errors.Is(err, ErrInsufficientTurns) {
		t.Fatalf("expected ErrInsufficientTurns, got %v", err)
	}
}

func TestRefillOverTime(t *testing.T) {
	now := time.Now()
	a := New(10, time.Minute, now)

	a, err := a.Spend(now, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Remaining != 0 {
		t.Fatalf("expected 0 remaining, got %d", a.Remaining)
	}

	later := now.Add(3*time.Minute + 30*time.Second)
	a = a.Refilled(later)
	if a.Remaining != 3 {
		t.Fatalf("expected 3 refilled turns, got %d", a.Remaining)
	}
	if !a.LastRefillAt.Equal(now.Add(3 * time.Minute)) {
		t.Fatalf("expected LastRefillAt to advance by whole ticks, got %v", a.LastRefillAt)
	}
}

func TestRefillCapsAtMax(t *testing.T) {
	now := time.Now()
	a := New(5, time.Minute, now)

	a, err := a.Spend(now, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	later := now.Add(time.Hour)
	a = a.Refilled(later)
	if a.Remaining != 5 {
		t.Fatalf("expected refill to cap at max 5, got %d", a.Remaining)
	}
}

func TestSpendRefillsBeforeSpending(t *testing.T) {
	now := time.Now()
	a := New(5, time.Minute, now)

	a, err := a.Spend(now, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	later := now.Add(2 * time.Minute)
	a, err = a.Spend(later, 2)
	if err != nil {
		t.Fatalf("unexpected error after refill: %v", err)
	}
	if a.Remaining != 0 {
		t.Fatalf("expected 0 remaining after refill+spend, got %d", a.Remaining)
	}
}
