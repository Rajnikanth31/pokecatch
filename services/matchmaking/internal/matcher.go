// Package matchmaking implements region-aware, MMR-banded pairing. The pool is a
// Redis sorted set per (region, mode) keyed by rating; this package contains the
// pure pairing policy so it can be unit-tested without Redis.
package matchmaking

import (
	"sort"
	"time"
)

// Ticket is a queued player. WaitStarted lets the band widen over time so nobody
// waits forever for a perfect match — a core fairness/latency tradeoff.
type Ticket struct {
	AccountID   string
	MMR         int
	RD          float64 // rating deviation; high RD => wider acceptable band
	Region      string
	Mode        string
	WaitStarted time.Time
}

// Match is a produced pairing.
type Match struct {
	A, B    Ticket
	BandUsed int
}

// Policy holds the tunables. Defaults give a snappy yet fair experience: start
// tight (±50), widen 25 MMR/sec, cap at ±400, and after 30s allow any opponent.
type Policy struct {
	BaseBand    int           // initial half-width in MMR
	WidenPerSec int           // MMR added to band per second waited
	MaxBand     int           // hard cap before "any opponent" kicks in
	HardTimeout time.Duration // after this, match with anyone in region
}

// DefaultPolicy is production's starting point, tuned from soft-launch data.
func DefaultPolicy() Policy {
	return Policy{BaseBand: 50, WidenPerSec: 25, MaxBand: 400, HardTimeout: 30 * time.Second}
}

// band returns the acceptable MMR half-width for a ticket given how long it has
// waited. RD scales it: a freshly-placed player (RD high) accepts a wider band
// because we are less certain of their true rating.
func (p Policy) band(t Ticket, now time.Time) int {
	waited := now.Sub(t.WaitStarted).Seconds()
	b := p.BaseBand + int(waited)*p.WidenPerSec
	b += int(t.RD / 4) // RD 350 (new) adds ~87 MMR of tolerance
	if b > p.MaxBand {
		b = p.MaxBand
	}
	return b
}

// Pair greedily produces matches from the current pool snapshot. It sorts by MMR
// and pairs nearest neighbours whose mutual bands overlap, which minimizes total
// rating disparity — a good-enough approximation of optimal matching that runs in
// O(n log n) and is trivial to reason about under load.
//
// Returns matches plus the still-unmatched tickets (to be left in the pool).
func Pair(pool []Ticket, p Policy, now time.Time) ([]Match, []Ticket) {
	byMode := map[string][]Ticket{}
	for _, t := range pool {
		key := t.Region + "|" + t.Mode
		byMode[key] = append(byMode[key], t)
	}
	var matches []Match
	var leftover []Ticket

	for _, group := range byMode {
		sort.Slice(group, func(i, j int) bool { return group[i].MMR < group[j].MMR })
		used := make([]bool, len(group))
		for i := 0; i < len(group); i++ {
			if used[i] {
				continue
			}
			best := -1
			bestGap := 1 << 30
			for j := i + 1; j < len(group); j++ {
				if used[j] {
					continue
				}
				gap := abs(group[i].MMR - group[j].MMR)
				// Match if either party's band covers the gap (waiting longest wins),
				// or either has exceeded the hard timeout.
				bi := p.band(group[i], now)
				bj := p.band(group[j], now)
				timedOut := now.Sub(group[i].WaitStarted) > p.HardTimeout || now.Sub(group[j].WaitStarted) > p.HardTimeout
				if (gap <= bi || gap <= bj || timedOut) && gap < bestGap {
					best, bestGap = j, gap
				}
				if group[j].MMR-group[i].MMR > bi && group[j].MMR-group[i].MMR > bj {
					break // sorted: no closer candidate further out
				}
			}
			if best >= 0 {
				used[i], used[best] = true, true
				matches = append(matches, Match{A: group[i], B: group[best], BandUsed: bestGap})
			}
		}
		for i, t := range group {
			if !used[i] {
				leftover = append(leftover, t)
			}
		}
	}
	return matches, leftover
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
