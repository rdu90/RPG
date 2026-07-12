// Package rng provides deterministic seeded random number generation shared
// by galaxy generation, economy pricing, and (later) combat/haggle
// resolution — anything that must be reproducible from a stored seed rather
// than relying on wall-clock entropy.
package rng

import "math/rand/v2"

// New returns a PRNG deterministically seeded from seed: the same seed
// always produces the same sequence, regardless of machine or run.
func New(seed int64) *rand.Rand {
	s := uint64(seed)
	return rand.New(rand.NewPCG(s, s^0x9E3779B97F4A7C15))
}
