// Internal test package (package apikey, not apikey_test) — unlike every
// other test file in this package, this one needs direct access to
// generate/parseCredential/verifySecret/hashSecret, which are
// deliberately unexported (format details, not part of the domain's
// public surface). Every other _test.go here stays package apikey_test,
// matching the rest of this codebase's convention.
package apikey

import (
	"strings"
	"testing"
)

func TestGenerate_ProducesExpectedLengths(t *testing.T) {
	gen, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(gen.keyID) != keyIDLen {
		t.Errorf("expected key_id length %d, got %d (%q)", keyIDLen, len(gen.keyID), gen.keyID)
	}
	if len(gen.rawSecret) != secretLen {
		t.Errorf("expected secret length %d, got %d (%q)", secretLen, len(gen.rawSecret), gen.rawSecret)
	}
	wantPrefix := "jln_live_" + gen.keyID[:4]
	if gen.keyPrefix != wantPrefix {
		t.Errorf("expected key_prefix %q, got %q", wantPrefix, gen.keyPrefix)
	}
	if gen.secretHash != hashSecret(gen.rawSecret) {
		t.Errorf("secret_hash does not match hashSecret(rawSecret)")
	}
	raw := rawCredential(gen.keyID, gen.rawSecret)
	if len(raw) != totalCredentialLen {
		t.Errorf("expected raw credential length %d, got %d (%q)", totalCredentialLen, len(raw), raw)
	}
}

func TestGenerate_KeyIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		gen, err := generate()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[gen.keyID] {
			t.Fatalf("duplicate key_id generated: %s", gen.keyID)
		}
		seen[gen.keyID] = true
	}
}

func TestParseCredential_RoundTrip(t *testing.T) {
	gen, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	raw := rawCredential(gen.keyID, gen.rawSecret)

	env, keyID, secret, ok := parseCredential(raw)
	if !ok {
		t.Fatalf("parseCredential(%q) failed to parse a value generate() itself produced", raw)
	}
	if env != envLive {
		t.Errorf("expected env %q, got %q", envLive, env)
	}
	if keyID != gen.keyID {
		t.Errorf("expected key_id %q, got %q", gen.keyID, keyID)
	}
	if secret != gen.rawSecret {
		t.Errorf("expected secret %q, got %q", gen.rawSecret, secret)
	}
}

// TestParseCredential_SecretContainingUnderscore is the case that proves
// why parseCredential uses fixed-width slicing rather than
// strings.Split(raw, "_"): base64.RawURLEncoding's alphabet legally
// contains '_', so a real generated secret can (and, over enough runs,
// will) contain one. A split-based parser would misparse this silently;
// this test keeps failing loudly if that regresses.
func TestParseCredential_SecretContainingUnderscore(t *testing.T) {
	const keyID = "abcdefghijkl" // 12 chars, well-formed but arbitrary
	// Secret deliberately contains '_' — a real base64url secret does,
	// often enough that a split-based parser would eventually see one.
	secret := "aaaa_bbbb_cccc_dddd_eeee_ffff_gggg_hhhh_iii" // 43 chars
	if len(secret) != secretLen {
		t.Fatalf("test fixture secret is %d chars, want %d", len(secret), secretLen)
	}
	if !strings.Contains(secret, "_") {
		t.Fatalf("test fixture secret must contain '_' to exercise the case being tested")
	}
	raw := rawCredential(keyID, secret)

	env, gotKeyID, gotSecret, ok := parseCredential(raw)
	if !ok {
		t.Fatalf("parseCredential(%q) failed on a secret containing '_'", raw)
	}
	if env != envLive || gotKeyID != keyID || gotSecret != secret {
		t.Errorf("parseCredential(%q) = (%q, %q, %q), want (%q, %q, %q)",
			raw, env, gotKeyID, gotSecret, envLive, keyID, secret)
	}
}

func TestParseCredential_RejectsMalformed(t *testing.T) {
	validKeyID := "abcdefghijkl"
	validSecret := strings.Repeat("a", secretLen)
	validRaw := rawCredential(validKeyID, validSecret)

	cases := map[string]string{
		"empty":                 "",
		"too short":             "jln_live_short",
		"too long":              validRaw + "x",
		"unknown environment":   "jln_prod_" + validKeyID + "_" + validSecret,
		"no jln_ prefix at all": strings.Repeat("x", totalCredentialLen),
		// Same total length as validRaw — replaces only the separator
		// character, so this specifically exercises the
		// rest[keyIDLen] != '_' branch rather than the length check.
		"missing separator before secret": "jln_live_" + validKeyID + "X" + validSecret,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, _, ok := parseCredential(raw); ok {
				t.Errorf("parseCredential(%q) unexpectedly succeeded", raw)
			}
		})
	}

	// jln_test_ must still be recognized as a well-formed environment,
	// even though this issue never ISSUES a test-environment credential
	// (TD §2: "diterima bentuknya, tidak pernah diterbitkan").
	testRaw := "jln_test_" + validKeyID + "_" + validSecret
	env, _, _, ok := parseCredential(testRaw)
	if !ok || env != envTest {
		t.Errorf("expected jln_test_ credential to parse with env=%q, got env=%q ok=%v", envTest, env, ok)
	}
}

func TestVerifySecret(t *testing.T) {
	gen, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !verifySecret(gen.rawSecret, gen.secretHash) {
		t.Error("expected verifySecret to accept the matching secret")
	}
	if verifySecret(gen.rawSecret+"x", gen.secretHash) {
		t.Error("expected verifySecret to reject a modified secret")
	}
	other, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if verifySecret(other.rawSecret, gen.secretHash) {
		t.Error("expected verifySecret to reject a different key's secret")
	}
}
