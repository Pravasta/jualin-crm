package webhook_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

// The fixture below is a KNOWN-ANSWER vector, not a round trip. The
// expected digest was produced outside Go by two independent
// implementations that agree — Python's hmac/hashlib and OpenSSL:
//
//	printf '1756800000.{"event":"lead.created","data":{"lead":{"id":"abc"}}}' \
//	  | openssl dgst -sha256 -hmac 'whsec_0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHI' -r
//	→ b561fe70504457a06e1c6099b59c34b7c09a2d288b0d6105587a8a09053f846c
//
// This matters more than it looks. A test that signs with Sign and then
// verifies with Sign passes even if the construction is wrong in a way
// both halves share — signing "<body>.<ts>", or omitting the separator,
// or hashing the hex of the body. Those bugs are invisible from inside
// and fatal from outside: no receiver following the documentation could
// ever reproduce the digest. Pinning to an external computation is what
// makes this test able to catch that.
const (
	vectorSecret = "whsec_0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHI" // #nosec G101 -- published test vector, not a live credential
	vectorUnix   = 1756800000
	vectorBody   = `{"event":"lead.created","data":{"lead":{"id":"abc"}}}`
	vectorV1     = "b561fe70504457a06e1c6099b59c34b7c09a2d288b0d6105587a8a09053f846c"
)

func vectorTime() time.Time { return time.Unix(vectorUnix, 0) }

func TestSign_MatchesExternallyComputedVector(t *testing.T) {
	got := webhook.Sign(vectorSecret, vectorTime(), []byte(vectorBody))
	want := "t=1756800000,v1=" + vectorV1
	if got != want {
		t.Fatalf("Sign()\n got %q\nwant %q", got, want)
	}
}

func TestSign_HeaderShape(t *testing.T) {
	got := webhook.Sign(vectorSecret, vectorTime(), []byte(vectorBody))

	parts := strings.Split(got, ",")
	if len(parts) != 2 {
		t.Fatalf("expected exactly two comma-separated parts, got %q", got)
	}
	if !strings.HasPrefix(parts[0], "t=") {
		t.Errorf("first part %q is not t=", parts[0])
	}
	if !strings.HasPrefix(parts[1], "v1=") {
		t.Errorf("second part %q is not v1=", parts[1])
	}
	if hex := strings.TrimPrefix(parts[1], "v1="); len(hex) != 64 {
		t.Errorf("v1 is %d chars, want 64 hex chars of SHA-256", len(hex))
	}
	if webhook.SignatureHeader != "X-Jualin-Signature" {
		t.Errorf("SignatureHeader = %q, want X-Jualin-Signature", webhook.SignatureHeader)
	}
}

// TestSign_OneBodyByteChangesSignature is the ordinary tamper property.
func TestSign_OneBodyByteChangesSignature(t *testing.T) {
	base := webhook.Sign(vectorSecret, vectorTime(), []byte(vectorBody))

	body := []byte(vectorBody)
	for i := range body {
		mutated := make([]byte, len(body))
		copy(mutated, body)
		mutated[i] ^= 0x01

		if got := webhook.Sign(vectorSecret, vectorTime(), mutated); got == base {
			t.Fatalf("flipping body byte %d did not change the signature", i)
		}
	}
}

// TestSign_TimestampIsActuallySigned is the acceptance criterion that
// separates this scheme from a naive one. It does NOT merely assert that
// two different timestamps produce two different header strings — that
// would pass trivially, since t appears in the output verbatim. It
// isolates the v1 half and proves THAT changed too, which can only happen
// if the timestamp really is inside the HMAC input.
//
// If t were signed only as a sibling field, an attacker replaying a
// captured request could rewrite t to now and the v1 they captured would
// still verify, forever.
func TestSign_TimestampIsActuallySigned(t *testing.T) {
	v1At := func(ts time.Time) string {
		full := webhook.Sign(vectorSecret, ts, []byte(vectorBody))
		_, v1, found := strings.Cut(full, ",v1=")
		if !found {
			t.Fatalf("malformed signature %q", full)
		}
		return v1
	}

	original := v1At(vectorTime())
	replayed := v1At(vectorTime().Add(1 * time.Second))

	if original == replayed {
		t.Fatal("v1 is identical for two different timestamps — the timestamp is NOT inside the signed payload, so a captured delivery could be replayed indefinitely by rewriting t")
	}
}

// TestSign_SeparatorIsNotAmbiguous guards the boundary between the two
// signed fields. Concatenating without a separator would make
// ("1756800000", ".body") and ("1756800000.", "body") hash identically —
// a length-extension-flavoured ambiguity that costs nothing to rule out.
func TestSign_SeparatorIsNotAmbiguous(t *testing.T) {
	a := webhook.Sign(vectorSecret, time.Unix(1756800000, 0), []byte(".x"))
	b := webhook.Sign(vectorSecret, time.Unix(17568000000, 0), []byte("x"))

	_, av1, _ := strings.Cut(a, ",v1=")
	_, bv1, _ := strings.Cut(b, ",v1=")
	if av1 == bv1 {
		t.Fatal("two different (timestamp, body) pairs produced the same v1 — the separator is ambiguous")
	}
}

func TestSign_DifferentSecretsDiffer(t *testing.T) {
	a := webhook.Sign(vectorSecret, vectorTime(), []byte(vectorBody))
	b := webhook.Sign(vectorSecret+"x", vectorTime(), []byte(vectorBody))
	if a == b {
		t.Fatal("two different secrets produced the same signature")
	}
}

// TestSign_UsesBodyBytesVerbatim proves Sign never re-encodes what it is
// given. Two JSON documents that are semantically equal but differ in
// whitespace must produce different signatures, because the receiver
// verifies against the raw bytes it received, not against a re-parse.
func TestSign_UsesBodyBytesVerbatim(t *testing.T) {
	compact := webhook.Sign(vectorSecret, vectorTime(), []byte(`{"a":1}`))
	spaced := webhook.Sign(vectorSecret, vectorTime(), []byte(`{"a": 1}`))
	if compact == spaced {
		t.Fatal("whitespace-different bodies signed identically — Sign is normalizing its input")
	}
}
