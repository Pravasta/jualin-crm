// Package apikey implements the credential external systems use to send
// leads into an organization (freeze 3.1, ADR-004). This issue (#46)
// covers only management — create/list/revoke as principal user
// (Owner/Admin). Authenticating WITH a credential this package issues,
// and the public POST /v1/leads endpoint that accepts it, are #47's
// scope; see the doc comments on generate/parseCredential/verifySecret
// below for the pieces built now but not yet wired to any HTTP path.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	KeyID                 string
	SecretHash            string
	KeyPrefix             string
	Name                  string
	Scopes                []string
	CreatedByMembershipID *uuid.UUID
	CreatedAt             time.Time
	LastUsedAt            *time.Time
	RevokedAt             *time.Time
	ExpiresAt             *time.Time
}

// ScopeLeadsWrite is the one scope value that exists in this codebase
// (ADR-004 aturan #4: the column exists from day one, values are added
// as endpoints that need them exist). It is both the only value Create
// accepts and the only value the CHECK constraint ck_api_keys_scopes
// permits — kept in sync manually since Postgres CHECK expressions can't
// reference a Go constant.
const ScopeLeadsWrite = "leads:write"

const (
	envLive = "live"
	envTest = "test"

	// keyIDRawBytes/secretRawBytes are byte counts BEFORE base64url
	// encoding. ADR-004 states "secret: 32char" and "entropi 256-bit" on
	// the same line, which cannot both hold (32 base64url characters is
	// ~192 bits, not 256). 256-bit is taken as correct — TD §2 records
	// this as a found inconsistency, not a silent fix (Rule #30): the
	// ADR's entire "SHA-256 is safe here" argument rests on 256-bit
	// entropy, so that is the number that must actually be produced.
	keyIDRawBytes  = 9  // -> 12 base64url characters, no padding
	secretRawBytes = 32 // -> 43 base64url characters, no padding (256-bit)

	keyIDLen  = 12
	secretLen = 43
	// credentialPrefixLen is len("jln_live_") == len("jln_test_") — both
	// environments share one length, which is what makes the fixed-width
	// parse in parseCredential possible without first locating a
	// separator.
	credentialPrefixLen = 9
	// totalCredentialLen = "jln_" + "live_"/"test_" + key_id + "_" + secret
	//                    =    4   +      5           +   12   +  1  +  43  = 65
	totalCredentialLen = credentialPrefixLen + keyIDLen + 1 + secretLen
)

// generated is generate's result. rawSecret is the only field that must
// never be persisted (Rule #21) — callers pass it out to the HTTP
// response exactly once and let it go out of scope immediately after.
type generated struct {
	keyID      string
	rawSecret  string
	secretHash string
	keyPrefix  string
}

// generate creates one new credential pair with crypto/rand. keyID is
// plaintext and becomes the lookup key (ADR-004); rawSecret is shown to
// the caller once and only its hash is stored.
func generate() (generated, error) {
	keyIDBytes := make([]byte, keyIDRawBytes)
	if _, err := rand.Read(keyIDBytes); err != nil {
		return generated{}, fmt.Errorf("apikey: generate key_id: %w", err)
	}
	keyID := base64.RawURLEncoding.EncodeToString(keyIDBytes)

	secretBytes := make([]byte, secretRawBytes)
	if _, err := rand.Read(secretBytes); err != nil {
		return generated{}, fmt.Errorf("apikey: generate secret: %w", err)
	}
	rawSecret := base64.RawURLEncoding.EncodeToString(secretBytes)

	return generated{
		keyID:      keyID,
		rawSecret:  rawSecret,
		secretHash: hashSecret(rawSecret),
		keyPrefix:  "jln_" + envLive + "_" + keyID[:4],
	}, nil
}

// hashSecret computes the same hash generate would have stored, so a raw
// secret received back from a client (#47) can be checked against it.
// SHA-256, not argon2id — ADR-004 explains why that's the correct choice
// for a crypto/rand secret and would be a mistake for a password.
func hashSecret(rawSecret string) string {
	sum := sha256.Sum256([]byte(rawSecret))
	return hex.EncodeToString(sum[:])
}

// verifySecret reports whether rawSecret hashes to hash, compared in
// constant time (ADR-004's verification steps, step 3). Not called from
// any HTTP path in this issue — #47's authentication usecase is the
// first real caller. Exists and is tested here because it is part of
// the credential FORMAT this issue owns, not part of the request
// handling #47 builds.
func verifySecret(rawSecret, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(hashSecret(rawSecret)), []byte(hash)) == 1
}

// rawCredential assembles the string shown to the caller exactly once —
// jln_live_<key_id>_<secret>. Built in one place so the create response
// and any future documentation example construct it identically.
func rawCredential(keyID, rawSecret string) string {
	return "jln_" + envLive + "_" + keyID + "_" + rawSecret
}

// parseCredential splits a raw jln_* credential into its parts using
// FIXED-WIDTH slicing, not strings.Split on "_". base64.RawURLEncoding
// uses '-' and '_' in place of '+' and '/', so key_id or secret can
// LEGALLY contain '_' — a split would silently misparse the moment
// either one did, which is exactly the case most likely to be exercised
// by real generated values (tested explicitly in entity_test.go). Total
// length is fixed (totalCredentialLen) because every field has a fixed
// encoded width, so length + index slicing fully determines validity
// without ever searching for a delimiter.
func parseCredential(raw string) (env, keyID, rawSecret string, ok bool) {
	if len(raw) != totalCredentialLen {
		return "", "", "", false
	}
	switch raw[:credentialPrefixLen] {
	case "jln_" + envLive + "_":
		env = envLive
	case "jln_" + envTest + "_":
		env = envTest
	default:
		return "", "", "", false
	}
	rest := raw[credentialPrefixLen:]
	if rest[keyIDLen] != '_' {
		return "", "", "", false
	}
	return env, rest[:keyIDLen], rest[keyIDLen+1:], true
}
