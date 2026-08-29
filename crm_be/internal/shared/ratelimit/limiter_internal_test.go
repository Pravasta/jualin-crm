package ratelimit

// package ratelimit (not ratelimit_test) — these tests need to set the
// unexported `now` field directly to drive generation rollover
// deterministically. Every other test in this package stays external
// (limiter_test.go); this is the same deliberate, isolated exception
// internal/apikey/entity_test.go already sets a precedent for (Phase 4
// #46) — access to an unexported detail, not a general pattern.

import (
	"fmt"
	"testing"
	"time"
)

// TestFixedWindow_EvictsAcrossGenerations is Phase 4.5 #58's direct
// proof that memory is bounded — measured, not read from the code
// (PRD Phase 4.5 acceptance criterion #5). Twenty generations of 50
// never-repeated keys each (the shape of an attacker who never reuses
// an IP/email) would accumulate 1000 tracked keys without eviction;
// with it, only the current and previous generation are ever held at
// once.
func TestFixedWindow_EvictsAcrossGenerations(t *testing.T) {
	l := NewFixedWindow(1, time.Minute)
	clock := time.Now()
	l.now = func() time.Time { return clock }

	const keysPerGeneration = 50
	const generations = 20

	maxLen := 0
	for g := 0; g < generations; g++ {
		for i := 0; i < keysPerGeneration; i++ {
			l.Take(fmt.Sprintf("gen%d-key%d", g, i))
		}
		if n := l.Len(); n > maxLen {
			maxLen = n
		}
		clock = clock.Add(time.Minute) // advance exactly one window
	}

	if maxLen > 2*keysPerGeneration {
		t.Errorf("expected tracked keys to stay near %d (current + previous generation), peaked at %d — memory is not bounded", 2*keysPerGeneration, maxLen)
	}
	if final := l.Len(); final > 2*keysPerGeneration {
		t.Errorf("expected final tracked key count to stay near %d, got %d", 2*keysPerGeneration, final)
	}
}

// TestFixedWindow_LiveWindowSurvivesGenerationSwap proves eviction is
// invisible to a key whose own window is still live — the exact
// property that keeps Phase 4.5 #58 from changing any existing rate
// limit's observable behavior (PRD acceptance criterion #7). Without
// this, a generation swap could silently hand an in-progress window a
// fresh budget, weakening the very limits #57 just finished proving
// can't be bypassed.
func TestFixedWindow_LiveWindowSurvivesGenerationSwap(t *testing.T) {
	l := NewFixedWindow(3, time.Minute)
	start := time.Now()
	clock := start
	l.now = func() time.Time { return clock }

	l.Take("seed") // establishes genStart = start

	// K's own window starts partway through this generation, so it
	// outlives the generation itself once a swap happens.
	clock = start.Add(30 * time.Second)
	r1 := l.Take("K")
	if !r1.Allowed || r1.Remaining != 2 {
		t.Fatalf("expected K's first call allowed with Remaining=2, got %+v", r1)
	}
	wantResetAt := clock.Add(time.Minute) // start+90s, K's own window end

	// Past genStart+window (start+60s) — the next Take triggers a
	// generation swap. K's own window (ends start+90s) is still live.
	clock = start.Add(61 * time.Second)
	r2 := l.Take("K")

	if !r2.Allowed {
		t.Fatalf("expected K's second call to still be allowed across the generation swap, got %+v", r2)
	}
	if r2.Remaining != 1 {
		t.Errorf("expected Remaining to keep decreasing (1) as if no swap happened, got %d — the generation swap reset K's count", r2.Remaining)
	}
	if !r2.ResetAt.Equal(wantResetAt) {
		t.Errorf("expected ResetAt to stay %v (K's own window, unaffected by the swap), got %v", wantResetAt, r2.ResetAt)
	}
}
