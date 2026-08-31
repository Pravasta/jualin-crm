// Package formtoken implements the embed page's time-trap token (Phase
// 6 #87, TD §6) — a stateless HMAC, not an anti-replay mechanism. A
// valid token can be resubmitted any number of times within its window;
// what actually bounds repetition is the rate limiter, not this
// package. This package answers exactly one question: "was this
// request issued between 2 seconds and 30 minutes ago, for this exact
// form?" — nothing else.
//
// Deliberately keyed by its own FORM_TOKEN_SECRET, never JWT_SECRET
// (internal/shared/config) — two unrelated purposes never share a key,
// so rotating one is never forced to rotate the other.
//
// Issue has zero callers as of #87 — the embed page that calls it is
// #88's. Built now anyway because Verify needs a real, non-hypothetical
// token shape to test against, and #88 needs the exact same shape
// Verify expects; splitting them across two issues would risk the two
// ends drifting. Same precedent as form.Repository.FindByPublicKey
// (#85, wired by #87).
package formtoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// minAge/maxAge are TD §6's time-trap window: a submission faster than
// a human filling a form (bot filled it instantly) or older than a
// plausible single sitting (token leaked/reused far later) is rejected.
// Not tunable via env — TD §6 states these two numbers directly as part
// of the mechanism's definition, unlike the rate-limit budgets, which
// are explicitly configurable because they're guesses.
const (
	minAge = 2 * time.Second
	maxAge = 30 * time.Minute
)

// ErrInvalidToken is the ONE error Verify ever returns for a bad token
// — wrong signature, too fast, expired, or malformed all collapse into
// this single sentinel, matching TD §9's single error code
// (form_token_invalid). Distinguishing "too fast" from "bad signature"
// to a caller would leak information about the check itself for no
// benefit — same reasoning apikey.ResolveAPIKey collapses every
// resolution failure into one identical 401.
var ErrInvalidToken = errors.New("formtoken: invalid or expired token")

// Issue mints a new token for formID, timestamped now. secret is
// FORM_TOKEN_SECRET, raw bytes (not base64-decoded) — config.go already
// enforces it's at least 32 bytes (Aturan #36).
func Issue(secret []byte, formID uuid.UUID) string {
	return issueAt(secret, formID, time.Now())
}

func issueAt(secret []byte, formID uuid.UUID, issuedAt time.Time) string {
	ts := strconv.FormatInt(issuedAt.Unix(), 10)
	sig := sign(secret, formID, ts)
	return base64.RawURLEncoding.EncodeToString([]byte(ts)) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// Verify reports whether token is a genuine, still-in-window token for
// formID. secret must be the SAME FORM_TOKEN_SECRET Issue was called
// with — a token issued for one form_id is rejected for any other
// (TD §6, and #87's own acceptance criterion "token form lain
// ditolak"), even with a byte-identical secret.
func Verify(secret []byte, token string, formID uuid.UUID) error {
	return verifyAt(secret, token, formID, time.Now())
}

func verifyAt(secret []byte, token string, formID uuid.UUID, now time.Time) error {
	tsPart, sigPart, ok := strings.Cut(token, ".")
	if !ok {
		return ErrInvalidToken
	}
	tsBytes, err := base64.RawURLEncoding.DecodeString(tsPart)
	if err != nil {
		return ErrInvalidToken
	}
	ts := string(tsBytes)
	issuedAtUnix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return ErrInvalidToken
	}

	gotSig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return ErrInvalidToken
	}
	// Constant-time comparison (hmac.Equal, not ==/bytes.Equal) — same
	// discipline apikey.verifySecret already applies to its own HMAC-
	// adjacent comparison (Aturan #20's reasoning extended to this
	// signature, even though TD §2 is explicit public_key itself needs
	// no such comparison; a forged SIGNATURE is exactly the threat this
	// one guards against).
	if !hmac.Equal(sign(secret, formID, ts), gotSig) {
		return ErrInvalidToken
	}

	age := now.Sub(time.Unix(issuedAtUnix, 0))
	if age < minAge || age > maxAge {
		return ErrInvalidToken
	}
	return nil
}

func sign(secret []byte, formID uuid.UUID, ts string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(formID.String() + "|" + ts))
	return mac.Sum(nil)
}
