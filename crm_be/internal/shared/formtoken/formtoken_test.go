// Internal test package (package formtoken, not formtoken_test) — needs
// issueAt/verifyAt to control "now" deterministically for the <2s/>30m
// boundary tests. Every other test in this package could live in
// formtoken_test instead; kept together here since splitting the file
// in two for that one distinction isn't worth it (Rule #27).
package formtoken

import (
	"encoding/base64"
	"strings"
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

// TestVerify_TamperedSignature_Rejected found a genuine flaky-test bug
// during CI, not a formtoken bug: flipping the LAST character of the
// signature's base64url encoding is unreliable. A 32-byte SHA-256
// digest's final symbol only carries 4 meaningful bits (32 mod 3 = 2
// leftover bytes → a trailing symbol with 2 discarded padding bits,
// standard base64 tail math) — 'A' and 'B' happen to differ ONLY in
// that discarded low bit, so whenever the real last character already
// shared the same top 4 bits as 'A' (a 4-in-64 chance per random
// signature — true for original characters 'A','B','C','D'), the
// "tamper" silently decoded back to the SAME bytes and the test failed
// non-deterministically (reproduced locally at ~6% with -count=500,
// matching that math exactly). Fixed by flipping the FIRST character of
// the signature instead — guaranteed to sit inside one of the digest's
// ten full 3-byte-to-4-symbol groups, where every bit of every symbol
// is meaningful, so any character change is guaranteed to change real
// signature bytes.
func TestVerify_TamperedSignature_Rejected(t *testing.T) {
	formID := uuid.Must(uuid.NewV7())
	issuedAt := time.Now().Truncate(time.Second) // matches the second-granularity issueAt/verifyAt actually store, so test offsets are exact, not fuzzed by truncation
	token := issueAt(testSecret, formID, issuedAt)

	sep := strings.IndexByte(token, '.')
	if sep < 0 || sep+1 >= len(token) {
		t.Fatalf("test fixture broken: expected token to contain '.' followed by a signature, got %q", token)
	}
	sigStart := sep + 1
	tampered := token[:sigStart] + flipChar(token[sigStart]) + token[sigStart+1:]
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
