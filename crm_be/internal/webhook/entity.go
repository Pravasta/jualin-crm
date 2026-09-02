// Package webhook implements outbound webhook delivery (Phase 7).
//
// #100 built management — CRUD of webhook_endpoints as principal user
// (Owner/Admin) — plus pieces their callers only arrive for later:
// backoff (pure, used by the worker in #102) and the DeliveryRepository
// claim/reap/purge methods (exercised by repository_test.go now, wired
// to the worker in #102 — precedent form.FindByPublicKey built in #85).
//
// #101 added signature.go, Enqueue, and the lead trigger, and corrected
// how the signing secret is stored: #100 hashed it, copying api_key,
// which would have made signing impossible (migration 0009).
//
// The worker loop and outbound HTTP are #102. Nothing in this package
// makes a network request yet.
package webhook

import (
	"crypto/rand"
	"encoding/base64"
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
	ID             uuid.UUID
	OrganizationID uuid.UUID
	URL            string
	// SecretCiphertext is the signing secret sealed with AES-256-GCM
	// (internal/shared/crypter), NOT a hash — migration 0009 explains at
	// length why this credential is the one that must be readable again.
	// It never leaves this package except to the database: the handler
	// serializes Endpoint without it, and the only reader is the worker
	// (#102), which decrypts it to compute a signature and drops it.
	SecretCiphertext      []byte
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

// ClaimedDelivery is one row the worker has taken ownership of, carrying
// everything a send needs so there is no second lookup between the claim
// and the HTTP call — a lookup that could race with the endpoint being
// edited or soft-deleted in between.
//
// EndpointSecretCiphertext lives HERE and not on Delivery on purpose.
// Delivery is what the dashboard reads and handler_http serializes; a
// signing secret, even sealed, has no business on a struct that gets
// marshalled to a response. Only the worker ever sees this type.
type ClaimedDelivery struct {
	Delivery
	EndpointURL              string
	EndpointSecretCiphertext []byte
}

// DeliveryResult is what the worker writes back after one attempt.
//
// ErrorText is deliberately short and OURS — never the receiver's response
// body. Their body may contain anything, including their own customers'
// data, and we have no reason to store it (TD §4.3, Rule #26's spirit).
type DeliveryResult struct {
	Status         string
	Attempt        int
	NextAttemptAt  time.Time
	ResponseStatus *int
	ErrorText      *string
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
	// entropy as an API key secret (apikey.secretRawBytes). Here the
	// reason is different from there: this value is an HMAC-SHA256 key,
	// and 256 bits is that construction's full-strength key size.
	secretRawBytes = 32
	secretPrefix   = "whsec_"
	// secretPrefixDisplayLen is how many characters of the encoded secret
	// are stored in secret_prefix for display in the endpoint list — long
	// enough to tell two secrets apart, short enough to be useless alone.
	secretPrefixDisplayLen = 8
)

// generatedSecret is generateSecret's result. rawSecret goes two places
// and no third: to the HTTP response exactly once (Rule #21), and into
// crypter.Encrypt on its way to secret_ciphertext. It is never logged and
// never stored in plaintext.
//
// There is no hash field. Through #100 there was one, and it was the
// defect migration 0009 corrects: a SHA-256 hash cannot serve as the HMAC
// key the worker needs, so the product could never have signed anything.
type generatedSecret struct {
	rawSecret string
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
	return generatedSecret{
		rawSecret: secretPrefix + encoded,
		prefix:    secretPrefix + encoded[:secretPrefixDisplayLen],
	}, nil
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

// MaxAttempts is the number of RETRIES after the initial delivery — not
// the total number of sends. The default therefore means up to 6 HTTP
// calls: one immediate, then five spaced by every delay in retryDelays.
//
// prd.md D5 says "5 percobaan" alongside a list of five delays, which only
// adds up under this reading: five attempts would leave the 6h delay
// unreachable. Resolved with the product owner and D5's wording corrected
// rather than left ambiguous (docs/issues/102).
const MaxAttempts = len(retryDelays)

// backoff returns how long to wait before the given RETRY number
// (1-indexed: backoff(1) is the wait after the initial send fails,
// backoff(MaxAttempts) the wait before the last retry). attempt <= 0 or
// beyond the table returns the nearest end — a caller should check
// attempt <= MaxAttempts before scheduling another retry at all, since
// past that the delivery is finished, not merely slower.
func backoff(attempt int) time.Duration {
	if attempt < 1 {
		return retryDelays[0]
	}
	if attempt > len(retryDelays) {
		return retryDelays[len(retryDelays)-1]
	}
	return retryDelays[attempt-1]
}
