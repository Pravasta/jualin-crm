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
