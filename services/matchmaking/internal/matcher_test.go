package matchmaking

import (
	"testing"
	"time"
)

func tk(id string, mmr int, region string, waited time.Duration) Ticket {
	return Ticket{AccountID: id, MMR: mmr, RD: 60, Region: region, Mode: "ranked", WaitStarted: time.Now().Add(-waited)}
}

func TestPairsNearestMMRWithinBand(t *testing.T) {
	now := time.Now()
	pool := []Ticket{
		tk("a", 1000, "eu", 0),
		tk("b", 1030, "eu", 0), // 30 apart -> within base band 50
		tk("c", 2000, "eu", 0), // far away
	}
	matches, leftover := Pair(pool, DefaultPolicy(), now)
	if len(matches) != 1 {
		t.Fatalf("want 1 match, got %d", len(matches))
	}
	m := matches[0]
	if !((m.A.AccountID == "a" && m.B.AccountID == "b") || (m.A.AccountID == "b" && m.B.AccountID == "a")) {
		t.Fatalf("expected a&b paired, got %s & %s", m.A.AccountID, m.B.AccountID)
	}
	if len(leftover) != 1 || leftover[0].AccountID != "c" {
		t.Fatalf("expected c left over, got %+v", leftover)
	}
}

func TestRegionIsolation(t *testing.T) {
	now := time.Now()
	pool := []Ticket{tk("a", 1000, "eu", 0), tk("b", 1000, "na", 0)}
	matches, leftover := Pair(pool, DefaultPolicy(), now)
	if len(matches) != 0 {
		t.Fatalf("cross-region match should not happen, got %d", len(matches))
	}
	if len(leftover) != 2 {
		t.Fatalf("both should remain queued, got %d", len(leftover))
	}
}

func TestBandWidensWithWait(t *testing.T) {
	now := time.Now()
	// 300 MMR apart: outside base band, but with 20s wait band ~ 50+500+15 > 300.
	pool := []Ticket{tk("a", 1000, "eu", 20*time.Second), tk("b", 1300, "eu", 20*time.Second)}
	matches, _ := Pair(pool, DefaultPolicy(), now)
	if len(matches) != 1 {
		t.Fatalf("waiting players should match across a wider band, got %d", len(matches))
	}
}

func TestHardTimeoutMatchesAnyone(t *testing.T) {
	now := time.Now()
	pool := []Ticket{tk("a", 1000, "eu", 40*time.Second), tk("b", 4000, "eu", 40*time.Second)}
	matches, _ := Pair(pool, DefaultPolicy(), now)
	if len(matches) != 1 {
		t.Fatalf("past hard timeout players should match regardless of gap, got %d", len(matches))
	}
}
