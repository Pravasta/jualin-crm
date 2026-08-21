package auth_test

import (
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
)

func TestLoginLimiter_AllowsFirstAttempt(t *testing.T) {
	l := auth.NewLoginLimiter()

	if ok, _ := l.Allow("k"); !ok {
		t.Error("expected the first attempt for a fresh key to be allowed")
	}
}

func TestLoginLimiter_BacksOffAfterFailure(t *testing.T) {
	l := auth.NewLoginLimiter()

	l.RecordFailure("k")

	ok, retryAfter := l.Allow("k")
	if ok {
		t.Error("expected the immediately-following attempt to be blocked")
	}
	if retryAfter <= 0 {
		t.Error("expected a positive retryAfter when blocked")
	}
}

func TestLoginLimiter_ResetsOnSuccess(t *testing.T) {
	l := auth.NewLoginLimiter()

	l.RecordFailure("k")
	l.RecordSuccess("k")

	if ok, _ := l.Allow("k"); !ok {
		t.Error("expected a successful login to clear the backoff entirely")
	}
}

func TestLoginLimiter_IndependentPerKey(t *testing.T) {
	l := auth.NewLoginLimiter()

	l.RecordFailure("attacker-ip")

	if ok, _ := l.Allow("victim-email"); !ok {
		t.Error("expected a different key's backoff to be unaffected")
	}
}

// TestLoginLimiter_NeverPermanentlyLocked is the explicit TD requirement:
// "tanpa lockout permanen". Backoff must be bounded, not ever-growing.
func TestLoginLimiter_NeverPermanentlyLocked(t *testing.T) {
	l := auth.NewLoginLimiter()

	for range 20 {
		l.RecordFailure("k")
	}

	_, retryAfter := l.Allow("k")
	const cap = 5 * 60 // 5 minutes, in seconds — must match loginBackoffCap
	if retryAfter.Seconds() > cap {
		t.Errorf("expected retryAfter to be capped at %ds, got %v", cap, retryAfter)
	}
}
