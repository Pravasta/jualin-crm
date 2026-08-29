package auth

// package auth (not auth_test) — these tests need to set the unexported
// `now` field directly to drive generation rollover deterministically.
// Every other LoginLimiter test stays external (login_limiter_test.go);
// this mirrors the same isolated exception ratelimit's own
// limiter_internal_test.go takes for the identical reason (Phase 4.5
// #58).

import (
	"fmt"
	"testing"
	"time"
)

// TestLoginLimiter_EvictsExpiredBackoff is Phase 4.5 #58's direct proof
// for LoginLimiter: an attacker who never repeats a key (rotating IP or
// email per attempt, exactly what #57 proved doesn't bypass the limit
// itself) must not grow this map without bound. Twenty generations of 50
// never-repeated, already-expired keys each would accumulate 1000
// tracked entries without eviction.
func TestLoginLimiter_EvictsExpiredBackoff(t *testing.T) {
	l := NewLoginLimiter()
	clock := time.Now()
	l.now = func() time.Time { return clock }

	const keysPerGeneration = 50
	const generations = 20

	maxLen := 0
	for g := 0; g < generations; g++ {
		for i := 0; i < keysPerGeneration; i++ {
			l.RecordFailure(fmt.Sprintf("gen%d-key%d", g, i))
		}
		if n := l.Len(); n > maxLen {
			maxLen = n
		}
		clock = clock.Add(loginBackoffCap) // advance past this generation's window
	}

	if maxLen > 2*keysPerGeneration {
		t.Errorf("expected tracked keys to stay near %d (current + previous generation), peaked at %d — memory is not bounded", 2*keysPerGeneration, maxLen)
	}
	if final := l.Len(); final > 2*keysPerGeneration {
		t.Errorf("expected final tracked key count to stay near %d, got %d", 2*keysPerGeneration, final)
	}
}

// TestLoginLimiter_ActiveBackoffSurvivesGenerationSwap proves eviction
// never shortens a backoff that's still in force — the property that
// keeps #58 from weakening the exact protection #57 just finished
// proving can't be bypassed (PRD Phase 4.5 acceptance criterion #3:
// active backoff survives a generation swap).
func TestLoginLimiter_ActiveBackoffSurvivesGenerationSwap(t *testing.T) {
	l := NewLoginLimiter()
	start := time.Now()
	clock := start
	l.now = func() time.Time { return clock }

	l.RecordFailure("seed") // establishes genStart = start

	// K's own backoff starts partway through this generation. A single
	// failure backs off by loginBackoffBase (1s), so K stays blocked
	// until start+31s.
	clock = start.Add(30 * time.Second)
	l.RecordFailure("K")

	// Past genStart+cap (start+5m) — the next call triggers a generation
	// swap. K's own backoff (until start+30s+1s) has long since expired
	// by real time, so pick a moment BEFORE that expiry instead: back off
	// with enough failures that K's backoff still covers the swap point.
	// A second failure doubles the delay to 2s, still trivially short —
	// so record failures until the capped delay (5m) safely outlasts the
	// gap to the swap.
	for i := 0; i < 10; i++ {
		l.RecordFailure("K")
	}
	// K's backoff is now capped at loginBackoffCap (5m) from start+30s,
	// i.e. active until start+30s+5m.

	clock = start.Add(loginBackoffCap + time.Second) // triggers the swap; K's backoff is still active
	ok, retryAfter := l.Allow("K")

	if ok {
		t.Fatal("expected K's active backoff to survive the generation swap, got allowed")
	}
	if retryAfter <= 0 {
		t.Errorf("expected a positive retryAfter for K's still-active backoff, got %v", retryAfter)
	}
}
