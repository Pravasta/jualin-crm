// Package token generates single-use secrets for email verification,
// password reset, and invitations. The raw value is what's emailed to
// the user; only its hash is ever stored (same principle as API keys,
// ADR-004) so a database read alone can never produce a usable token.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const rawLength = 32 // bytes, before encoding

// Generate returns a random raw token and its SHA-256 hash (hex-encoded).
// Store hash; email raw; never store raw.
func Generate() (raw string, hash string, err error) {
	b := make([]byte, rawLength)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("token: failed to generate: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, Hash(raw), nil
}

// Hash computes the same hash Generate would have stored, so callers can
// verify a raw token received from a client against a stored hash.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
