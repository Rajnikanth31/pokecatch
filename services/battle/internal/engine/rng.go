package engine

import "math"

// RNG is the engine's only randomness source. It is server-authoritative and
// SEEDED PER MATCH so the entire battle is a pure function of (seed, inputs).
// This gives us three things at once:
//  1. Deterministic replays for spectating and dispute resolution.
//  2. Cheap anti-cheat: the server re-simulates from the seed and compares the
//     resulting state hash to anything a client claims.
//  3. Reproducible unit tests.
//
// It is a SplitMix64 generator: tiny, fast, and statistically solid for game
// use (we are not doing cryptography). The seed is generated server-side with
// crypto/rand at match start and never sent to clients until the match ends.
type RNG struct {
	state uint64
}

// NewRNG constructs a generator from a 64-bit match seed.
func NewRNG(seed uint64) *RNG { return &RNG{state: seed} }

func (r *RNG) next() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// IntN returns a uniform integer in [0, n). Panics on n<=0 to surface logic bugs
// early rather than silently bias.
func (r *RNG) IntN(n int) int {
	if n <= 0 {
		panic("engine: RNG.IntN requires n > 0")
	}
	return int(r.next() % uint64(n))
}

// Chance returns true with probability pct/100. pct<=0 is always false, pct>=100
// always true, so callers never need to special-case guaranteed effects.
func (r *RNG) Chance(pct int) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	return r.IntN(100) < pct
}

// Float returns a uniform float in [0,1).
func (r *RNG) Float() float64 {
	return float64(r.next()>>11) / float64(uint64(1)<<53)
}

// State exposes the current internal state so it can be hashed into the
// authoritative match digest.
func (r *RNG) State() uint64 { return r.state }

// roundHalfUp is shared rounding used across the damage pipeline so behaviour is
// identical on every platform (Go's math.Round is half-away-from-zero, which is
// what we want for non-negative damage).
func roundHalfUp(f float64) int { return int(math.Round(f)) }
