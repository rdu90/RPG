// Package turn implements the turn-allowance math: remaining/max turns that
// refill over real elapsed time, computed lazily from timestamps rather
// than a running background scheduler. Because Refilled and Spend are pure
// functions of (state, now), the same code resolves correctly whether it's
// backing a local save or a shared async-multiplayer server — the classic
// TradeWars BBS door-game shape.
package turn

import (
	"errors"
	"time"
)

// ErrInsufficientTurns is returned when a Spend would take Remaining below
// zero.
var ErrInsufficientTurns = errors.New("turn: insufficient turns remaining")

// Allowance is a turn budget that refills by one turn every RefillEvery,
// up to Max.
type Allowance struct {
	Max          int
	Remaining    int
	RefillEvery  time.Duration
	LastRefillAt time.Time
}

// New creates a full allowance as of now.
func New(max int, refillEvery time.Duration, now time.Time) Allowance {
	return Allowance{Max: max, Remaining: max, RefillEvery: refillEvery, LastRefillAt: now}
}

// Refilled returns a's state advanced to now: for every RefillEvery that
// has elapsed since LastRefillAt, Remaining increases by one turn, capped
// at Max.
func (a Allowance) Refilled(now time.Time) Allowance {
	if a.Remaining >= a.Max || a.RefillEvery <= 0 {
		return a
	}
	elapsed := now.Sub(a.LastRefillAt)
	if elapsed < a.RefillEvery {
		return a
	}
	ticks := int(elapsed / a.RefillEvery)

	a.Remaining += ticks
	if a.Remaining > a.Max {
		a.Remaining = a.Max
	}
	a.LastRefillAt = a.LastRefillAt.Add(time.Duration(ticks) * a.RefillEvery)
	return a
}

// Spend refills a as of now, then attempts to spend n turns. On success it
// returns the updated allowance; on failure it returns the refilled (but
// unspent) allowance and ErrInsufficientTurns.
func (a Allowance) Spend(now time.Time, n int) (Allowance, error) {
	a = a.Refilled(now)
	if n > a.Remaining {
		return a, ErrInsufficientTurns
	}
	a.Remaining -= n
	return a, nil
}
