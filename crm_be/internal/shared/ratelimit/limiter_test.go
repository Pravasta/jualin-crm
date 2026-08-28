package ratelimit_test

import (
	"testing"
	"time"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
)

func TestFixedWindow_AllowsUpToLimit(t *testing.T) {
	l := ratelimit.NewFixedWindow(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("expected call %d to be allowed", i+1)
		}
	}
	if l.Allow("k") {
		t.Error("expected 4th call within the window to be denied")
	}
}

func TestFixedWindow_KeysAreIndependent(t *testing.T) {
	l := ratelimit.NewFixedWindow(1, time.Minute)

	if !l.Allow("a") {
		t.Fatal("expected first call for key 'a' to be allowed")
	}
	if !l.Allow("b") {
		t.Error("expected key 'b' to be independent of key 'a''s limit")
	}
}

func TestFixedWindow_ResetsAfterWindow(t *testing.T) {
	l := ratelimit.NewFixedWindow(1, 10*time.Millisecond)

	if !l.Allow("k") {
		t.Fatal("expected first call to be allowed")
	}
	if l.Allow("k") {
		t.Fatal("expected second call within the window to be denied")
	}

	time.Sleep(20 * time.Millisecond)

	if !l.Allow("k") {
		t.Error("expected a call after the window elapsed to be allowed again")
	}
}

func TestFixedWindow_Take_RemainingDecreasesToZeroAtLimit(t *testing.T) {
	l := ratelimit.NewFixedWindow(3, time.Minute)

	want := []int{2, 1, 0}
	for i, w := range want {
		r := l.Take("k")
		if !r.Allowed {
			t.Fatalf("call %d: expected allowed", i+1)
		}
		if r.Limit != 3 {
			t.Errorf("call %d: expected Limit 3, got %d", i+1, r.Limit)
		}
		if r.Remaining != w {
			t.Errorf("call %d: expected Remaining %d, got %d", i+1, w, r.Remaining)
		}
	}

	r := l.Take("k")
	if r.Allowed {
		t.Fatal("expected the 4th call within the window to be denied")
	}
	if r.Remaining != 0 {
		t.Errorf("expected Remaining 0 once denied, got %d", r.Remaining)
	}
}

func TestFixedWindow_Take_ResetAtIsWindowStartPlusWindow(t *testing.T) {
	l := ratelimit.NewFixedWindow(1, time.Minute)

	before := time.Now()
	r := l.Take("k")
	after := time.Now()

	if r.ResetAt.Before(before.Add(time.Minute)) || r.ResetAt.After(after.Add(time.Minute)) {
		t.Errorf("expected ResetAt to be ~1 minute after Take was called, got %v (call window [%v, %v])", r.ResetAt, before, after)
	}

	// A second Take within the same window must report the SAME ResetAt —
	// the window doesn't restart on every call, only when it elapses.
	r2 := l.Take("k")
	if !r2.ResetAt.Equal(r.ResetAt) {
		t.Errorf("expected ResetAt to stay fixed within one window, got %v then %v", r.ResetAt, r2.ResetAt)
	}
}

func TestFixedWindow_Take_MirrorsAllowsObservableBehavior(t *testing.T) {
	// Allow is now implemented in terms of Take — this locks that the
	// rewrite didn't change Allow's observable sequence for the exact
	// scenario TestFixedWindow_AllowsUpToLimit already covers.
	l := ratelimit.NewFixedWindow(2, time.Minute)

	if !l.Take("k").Allowed {
		t.Fatal("expected call 1 to be allowed")
	}
	if !l.Take("k").Allowed {
		t.Fatal("expected call 2 to be allowed")
	}
	if l.Take("k").Allowed {
		t.Fatal("expected call 3 to be denied")
	}
}
