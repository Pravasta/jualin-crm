package webhook

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateSecret_Format(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		s, err := generateSecret()
		if err != nil {
			t.Fatalf("generateSecret: %v", err)
		}
		if !strings.HasPrefix(s.rawSecret, "whsec_") {
			t.Errorf("rawSecret %q missing whsec_ prefix", s.rawSecret)
		}
		// whsec_ (6) + 43 base64url chars (32 raw bytes, no padding)
		if len(s.rawSecret) != 6+43 {
			t.Errorf("rawSecret length = %d, want %d", len(s.rawSecret), 6+43)
		}
		if strings.Contains(s.rawSecret, "=") {
			t.Errorf("rawSecret %q contains base64 padding", s.rawSecret)
		}
		if !strings.HasPrefix(s.prefix, "whsec_") || len(s.prefix) != 6+8 {
			t.Errorf("prefix %q malformed", s.prefix)
		}
		if !strings.HasPrefix(s.rawSecret, s.prefix) {
			t.Errorf("prefix %q is not a prefix of rawSecret %q", s.prefix, s.rawSecret)
		}
		if s.hash == s.rawSecret || len(s.hash) != 64 {
			t.Errorf("hash %q looks wrong (want 64 hex chars, not the raw secret)", s.hash)
		}
		if hashSecret(s.rawSecret) != s.hash {
			t.Error("hash does not match hashSecret(rawSecret)")
		}
		if seen[s.rawSecret] {
			t.Fatalf("generateSecret produced a duplicate: %q", s.rawSecret)
		}
		seen[s.rawSecret] = true
	}
}

func TestBackoff_MatchesTDSchedule(t *testing.T) {
	// TD §5 / keputusan D5: 1m, 5m, 30m, 2h, 6h. attempt is 1-indexed.
	want := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		6 * time.Hour,
	}
	for i, w := range want {
		attempt := i + 1
		if got := backoff(attempt); got != w {
			t.Errorf("backoff(%d) = %s, want %s", attempt, got, w)
		}
	}
	if MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", MaxAttempts)
	}
}

func TestBackoff_ClampsOutOfRange(t *testing.T) {
	if got := backoff(0); got != 1*time.Minute {
		t.Errorf("backoff(0) = %s, want 1m (first delay)", got)
	}
	if got := backoff(-3); got != 1*time.Minute {
		t.Errorf("backoff(-3) = %s, want 1m", got)
	}
	if got := backoff(99); got != 6*time.Hour {
		t.Errorf("backoff(99) = %s, want 6h (last delay)", got)
	}
}

func TestKnownEvents_Closed(t *testing.T) {
	if len(KnownEvents) != 2 {
		t.Fatalf("KnownEvents has %d entries, want 2", len(KnownEvents))
	}
	want := map[string]bool{"lead.created": true, "lead.status_changed": true}
	for _, ev := range KnownEvents {
		if !want[ev] {
			t.Errorf("unexpected known event %q", ev)
		}
	}
}
