// Internal test package (package formtoken, not formtoken_test) — needs
// issueAt/verifyAt to control "now" deterministically for the <2s/>30m
// boundary tests. Every other test in this package could live in
// formtoken_test instead; kept together here since splitting the file
// in two for that one distinction isn't worth it (Rule #27).
package formtoken

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
)

var testSecret = []byte("test-form-token-secret-at-least-32-bytes")

func TestIssue_ProducesVerifiableToken(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	token := Issue(testSecret, formID)
	if token == "" {
		t.Fatal("expected a non-empty token")
	}

	// Issue stamps "now" internally — Verify (also real time) must
	// reject it as too-fast immediately after minting, proving the two
	// public functions round-trip against real wall-clock time, not
	// just the *At test helpers below.
	if err := Verify(testSecret, token, formID); err != ErrInvalidToken {
		t.Errorf("expected a freshly issued token to fail as too-fast, got: %v", err)
	}
}

func TestVerify_ValidWithinWindow_Succeeds(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	issuedAt := time.Now().Truncate(time.Second) // matches the second-granularity issueAt/verifyAt actually store, so test offsets are exact, not fuzzed by truncation
	token := issueAt(testSecret, formID, issuedAt)

	cases := []struct {
		name string
		at   time.Time
	}{
		{"just past the minimum age", issuedAt.Add(minAge + time.Second)},
		{"midway through the window", issuedAt.Add(15 * time.Minute)},
		{"just under the maximum age", issuedAt.Add(maxAge - time.Second)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := verifyAt(testSecret, token, formID, c.at); err != nil {
				t.Errorf("expected a valid token to verify, got: %v", err)
			}
		})
	}
}

func TestVerify_ExactBoundaries_AreInclusive(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	issuedAt := time.Now().Truncate(time.Second) // matches the second-granularity issueAt/verifyAt actually store, so test offsets are exact, not fuzzed by truncation
	token := issueAt(testSecret, formID, issuedAt)

	// age == minAge and age == maxAge are both valid — the checks are
	// age < minAge / age > maxAge, strictly outside the window.
	if err := verifyAt(testSecret, token, formID, issuedAt.Add(minAge)); err != nil {
		t.Errorf("expected age exactly at minAge to succeed, got: %v", err)
	}
	if err := verifyAt(testSecret, token, formID, issuedAt.Add(maxAge)); err != nil {
		t.Errorf("expected age exactly at maxAge to succeed, got: %v", err)
	}
}

func TestVerify_TooFast_Rejected(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	issuedAt := time.Now().Truncate(time.Second) // matches the second-granularity issueAt/verifyAt actually store, so test offsets are exact, not fuzzed by truncation
	token := issueAt(testSecret, formID, issuedAt)

	cases := []time.Duration{0, time.Second, minAge - time.Millisecond}
	for _, age := range cases {
		if err := verifyAt(testSecret, token, formID, issuedAt.Add(age)); err != ErrInvalidToken {
			t.Errorf("age %s: expected ErrInvalidToken (too fast), got: %v", age, err)
		}
	}
}

func TestVerify_Expired_Rejected(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	issuedAt := time.Now().Truncate(time.Second) // matches the second-granularity issueAt/verifyAt actually store, so test offsets are exact, not fuzzed by truncation
	token := issueAt(testSecret, formID, issuedAt)

	cases := []time.Duration{maxAge + time.Millisecond, maxAge + time.Hour, 24 * time.Hour}
	for _, age := range cases {
		if err := verifyAt(testSecret, token, formID, issuedAt.Add(age)); err != ErrInvalidToken {
			t.Errorf("age %s: expected ErrInvalidToken (expired), got: %v", age, err)
		}
	}
}

func TestVerify_WrongSecret_Rejected(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	issuedAt := time.Now().Truncate(time.Second) // matches the second-granularity issueAt/verifyAt actually store, so test offsets are exact, not fuzzed by truncation
	token := issueAt(testSecret, formID, issuedAt)

	wrongSecret := []byte("a-completely-different-secret-32-bytes!")
	if err := verifyAt(wrongSecret, token, formID, issuedAt.Add(minAge)); err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for a token verified against the wrong secret, got: %v", err)
	}
}

// TestVerify_TokenForAnotherForm_Rejected is #87's own acceptance
// criterion, verbatim: "token form lain ditolak" — a token minted for
// form A must not verify against form B, even with the identical
// secret and well within the time window.
func TestVerify_TokenForAnotherForm_Rejected(t *testing.T) {
	formA := uuid.Must(uuid.NewV7())
	formB := uuid.Must(uuid.NewV7())
	issuedAt := time.Now().Truncate(time.Second) // matches the second-granularity issueAt/verifyAt actually store, so test offsets are exact, not fuzzed by truncation
	token := issueAt(testSecret, formA, issuedAt)

	if err := verifyAt(testSecret, token, formB, issuedAt.Add(minAge)); err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for a token issued to a different form_id, got: %v", err)
	}
}

func TestVerify_MalformedToken_Rejected(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	cases := map[string]string{
		"empty":                 "",
		"no separator":          "notoken",
		"multiple separators":   "a.b.c",
		"non-base64 timestamp":  "not*valid*base64.c2ln",
		"non-base64 signature":  "dGltZXN0YW1w.not*valid*base64",
		"timestamp not numeric": base64.RawURLEncoding.EncodeToString([]byte("not-a-number")) + ".c2ln",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Verify(testSecret, token, formID); err != ErrInvalidToken {
				t.Errorf("expected ErrInvalidToken for a malformed token, got: %v", err)
			}
		})
	}
}

func TestVerify_TamperedSignature_Rejected(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	issuedAt := time.Now().Truncate(time.Second) // matches the second-granularity issueAt/verifyAt actually store, so test offsets are exact, not fuzzed by truncation
	token := issueAt(testSecret, formID, issuedAt)

	// Flip the token's last character — still valid base64url shape (in
	// almost every case), but the signature no longer matches.
	tampered := token[:len(token)-1] + flipChar(token[len(token)-1])
	if err := verifyAt(testSecret, tampered, formID, issuedAt.Add(minAge)); err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for a tampered signature, got: %v", err)
	}
}

func flipChar(b byte) string {
	if b == 'A' {
		return "B"
	}
	return "A"
}
