// Package password hashes and verifies user passwords with argon2id.
//
// This is deliberately a different algorithm choice from API keys
// (SHA-256, see ADR-004): passwords are chosen by humans and have low
// entropy, so a slow, memory-hard hash is what makes brute-forcing
// expensive. API keys are 256-bit random values where a slow hash buys
// nothing but latency.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parameters are stored inside the hash string itself (PHC format), so
// raising them later never requires a migration — verification reads
// whatever parameters produced the stored hash.
const (
	saltLength  = 16
	keyLength   = 32
	memoryKiB   = 19 * 1024 // 19 MiB
	iterations  = 2
	parallelism = 1
)

var ErrInvalidHash = errors.New("password: malformed hash")

// Hash produces a PHC-formatted argon2id hash:
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
func Hash(plaintext string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password: failed to generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plaintext), salt, iterations, memoryKiB, parallelism, keyLength)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// Verify reports whether plaintext matches the given PHC-formatted hash,
// using the parameters embedded in the hash itself — not the package's
// current constants — so a hash created with older parameters still
// verifies correctly.
func Verify(plaintext, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version, mem, iter, par int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iter, &par); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	// #nosec G115 -- mem/iter/par/keyLength are parsed from a hash this
	// package itself generated with bounded values; not attacker-controlled
	// in a way that risks overflow.
	got := argon2.IDKey([]byte(plaintext), salt, uint32(iter), uint32(mem), uint8(par), uint32(len(want)))

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
