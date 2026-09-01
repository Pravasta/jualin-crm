// Package webhook implements outbound webhook delivery (Phase 7). This
// issue (#100) covers management only — CRUD of webhook_endpoints as
// principal user (Owner/Admin) — plus the pieces the later issues need
// built ahead of their callers: generateSecret (the credential format,
// same precedent as apikey.generate built in #46), backoff (pure, used
// by the worker in #102), and the DeliveryRepository claim/reap/purge
// methods (exercised by repository_test.go now, wired to the worker in
// #102 — precedent form.FindByPublicKey built in #85).
//
// Signing a payload (signature.go), enqueuing a delivery, and the lead
// trigger are #101. The worker loop and HTTP delivery are #102.
package webhook

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event types this product emits (Phase 7 keputusan D3). Two — enough to
// prove the whole mechanism (queue, retry, signature, SSRF) while being
// genuinely useful (pipeline sync, not just a notification). A third
// event later is one new caller, not a new mechanism.
const (
	EventLeadCreated       = "lead.created"
	EventLeadStatusChanged = "lead.status_changed"
)

// KnownEvents is the closed set Create/Update validate against — a typo
// in a subscribed event name would otherwise be a silent subscription to
// nothing.
var KnownEvents = []string{EventLeadCreated, EventLeadStatusChanged}

// Delivery status values — mirror ck_webhook_deliveries_status.
const (
	StatusPending    = "pending"
	StatusDelivering = "delivering"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
)

type Endpoint struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	URL                   string
	SecretHash            string
	SecretPrefix          string
	Events                []string
	Description           string
	IsActive              bool
	CreatedByMembershipID *uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

// Delivery is one attempt-bearing row of webhook_deliveries — both a
// history record (read by the dashboard, #103) and a queue entry
// (claimed by the worker, #102).
type Delivery struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	EndpointID      uuid.UUID
	EventType       string
	Payload         []byte
	Status          string
	Attempt         int
	NextAttemptAt   time.Time
	ResponseStatus  *int
	Error           *string
	DeliveringSince *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UpdateInput is Repository.Update's argument — nil means "leave
// unchanged", same convention form.UpdateInput / customer.UpdateInput
// use. IsActive is *bool so "deactivate" (false) is distinguishable from
// "don't touch it" (nil).
type UpdateInput struct {
	URL         *string
	Events      *[]string
	Description *string
	IsActive    *bool
}

const (
	// secretRawBytes is 32 (256-bit) before base64url encoding — the same
	// entropy as an API key secret (apikey.secretRawBytes) and for the
	// same reason: the SHA-256 storage argument (Rule #20) rests on it.
	secretRawBytes = 32
	secretPrefix   = "whsec_"
	// secretPrefixDisplayLen is how many characters of the encoded secret
	// are stored in secret_prefix for display in the endpoint list — long
	// enough to tell two secrets apart, short enough to be useless alone.
	secretPrefixDisplayLen = 8
)

// generatedSecret is generateSecret's result. rawSecret is the only
// field that must never be persisted (Rule #21) — the create handler
// passes it to the HTTP response exactly once.
type generatedSecret struct {
	rawSecret string
	hash      string
	prefix    string
}

// generateSecret creates one signing secret with crypto/rand. Format
// whsec_<43 base64url chars>. The prefix is deliberately NOT jln_live_ or
// pk_ — this credential's trust direction is reversed (we prove ourselves
// to the receiver, TD §2), and three credentials with opposing rules must
// not look alike.
func generateSecret() (generatedSecret, error) {
	b := make([]byte, secretRawBytes)
	if _, err := rand.Read(b); err != nil {
		return generatedSecret{}, fmt.Errorf("webhook: generate secret: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(b)
	raw := secretPrefix + encoded
	return generatedSecret{
		rawSecret: raw,
		hash:      hashSecret(raw),
		prefix:    secretPrefix + encoded[:secretPrefixDisplayLen],
	}, nil
}

// hashSecret is SHA-256 hex — same as apikey.hashSecret. The secret is a
// crypto/rand value, never a password: argon2id would be the wrong choice
// here for the reason ADR-004 spells out for API keys.
func hashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// retryDelays is TD §5 / keputusan D5 verbatim: 5 attempts total, the
// gap AFTER attempt N is retryDelays[N-1]. Not a measured set of numbers
// — it joins the shared review in api.md's "Angka batasnya belum pernah
// diukur" (issue #98). What is NOT a guess is the shape: a 4xx means
// "your request is wrong", and repeating it changes nothing.
// An array, not a slice, so MaxAttempts can be a compile-time constant.
var retryDelays = [5]time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	6 * time.Hour,
}

// MaxAttempts is the number of delivery attempts before a delivery is
// marked failed permanently.
const MaxAttempts = len(retryDelays)

// backoff returns how long to wait before the given attempt number
// (1-indexed: attempt 1 is the first retry, scheduled retryDelays[0]
// after the initial send). attempt <= 0 or beyond the table returns the
// last delay — callers should check attempt < MaxAttempts before
// scheduling another retry at all.
func backoff(attempt int) time.Duration {
	if attempt < 1 {
		return retryDelays[0]
	}
	if attempt > len(retryDelays) {
		return retryDelays[len(retryDelays)-1]
	}
	return retryDelays[attempt-1]
}
