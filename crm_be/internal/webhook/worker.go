package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// reapThreshold is how long a row may sit 'delivering' before it is
// assumed abandoned (TD §4.2). Deliberately NOT configurable: it is not a
// tuning knob but a safety margin, and it is only correct while it stays
// far above WEBHOOK_DELIVERY_TIMEOUT. Exposing it would invite setting it
// below that, at which point one instance's reaper starts stealing another
// instance's genuinely in-flight deliveries and every one of them is sent
// twice.
const reapThreshold = 10 * time.Minute

// purgeInterval throttles retention (TD §10) — lazily, from inside the
// worker loop, with no scheduler. Same pattern as idempotency_key
// cleanup (#47), including its open question: neither has been exercised
// at production volume (docs/issues/047).
const purgeInterval = time.Hour

// drainLimit caps how much of a receiver's response body is read. Their
// body is never stored (TD §4.3) — this reads and discards a bounded
// amount purely so a receiver that always answers with a large body
// cannot make us hold a connection open, and stops us from reading an
// unbounded one into memory to achieve nothing.
const drainLimit = 4 << 10

// deliveryQueue is the slice of DeliveryRepository the worker actually
// uses — declared here, at the consumer, per ADR-011. Every method is
// cross-organization: the worker is infrastructure, and which tenant a
// claimed row belongs to is an output of the claim, never an input to it
// (TD §1.2).
type deliveryQueue interface {
	ClaimDue(ctx context.Context, limit int) ([]*ClaimedDelivery, error)
	MarkResult(ctx context.Context, id uuid.UUID, res DeliveryResult) error
	Release(ctx context.Context, id uuid.UUID) error
	Reap(ctx context.Context, threshold time.Time) (int, error)
	Purge(ctx context.Context, before time.Time) (int, error)
}

// WorkerConfig is the worker's half of TD §9. Values are validated at
// boot by internal/shared/config, not here.
type WorkerConfig struct {
	Interval        time.Duration
	Batch           int
	DeliveryTimeout time.Duration
	// MaxAttempts is the number of RETRIES after the initial send — see
	// the MaxAttempts constant.
	MaxAttempts   int
	RetentionDays int
}

// Worker is the product's first async infrastructure: a goroutine inside
// the api binary that drains webhook_deliveries (keputusan D2 — not a
// separate process, not a broker).
//
// It is single-threaded by construction. One goroutine runs every tick,
// so none of its state needs a mutex, and one slow endpoint delays its own
// batch rather than the queue (TD §13).
type Worker struct {
	queue   deliveryQueue
	client  *http.Client
	crypter SecretCrypter
	cfg     WorkerConfig
	logger  *slog.Logger

	// now is time.Now in production and a stub in tests, so backoff
	// scheduling can be asserted exactly rather than approximately.
	now func() time.Time

	lastPurge time.Time
}

func NewWorker(queue deliveryQueue, client *http.Client, crypter SecretCrypter, cfg WorkerConfig, logger *slog.Logger) *Worker {
	return &Worker{
		queue:   queue,
		client:  client,
		crypter: crypter,
		cfg:     cfg,
		logger:  logger,
		now:     time.Now,
	}
}

// Run drives the loop until ctx is cancelled, then returns. The caller
// (cmd/api) waits on that return, so a hung tick delays shutdown rather
// than being abandoned — which is what "no dangling goroutine" actually
// requires.
//
// A tick runs immediately on entry rather than after the first interval:
// on restart there may be rows a previous process left behind, and making
// them wait out an interval for no reason is the kind of small
// unnecessary latency that compounds across deploys.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("webhook worker started",
		"interval", w.cfg.Interval, "batch", w.cfg.Batch, "max_retries", w.cfg.MaxAttempts)

	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	for {
		w.tick(ctx)

		select {
		case <-ctx.Done():
			w.logger.Info("webhook worker stopped")
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	// Reaper first: rows a crashed or shut-down instance left claimed have
	// to be back in the queue before this tick claims, or they wait a full
	// extra interval for nothing.
	w.reap(ctx)
	w.maybePurge(ctx)

	claimed, err := w.queue.ClaimDue(ctx, w.cfg.Batch)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Error("webhook claim failed", "err", err)
		}
		return
	}

	for i, cd := range claimed {
		if ctx.Err() != nil {
			// Shutting down. Everything not yet attempted goes straight
			// back to the queue instead of waiting out the reaper's
			// threshold — otherwise every deploy delays these by ten
			// minutes. A fresh context: the parent is already cancelled,
			// and this write must still happen.
			w.releaseRemaining(claimed[i:])
			return
		}
		w.deliver(ctx, cd)
	}
}

func (w *Worker) reap(ctx context.Context) {
	n, err := w.queue.Reap(ctx, w.now().Add(-reapThreshold))
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Error("webhook reaper failed", "err", err)
		}
		return
	}
	if n > 0 {
		// Worth a log line every time: a nonzero count means a process
		// died mid-delivery, which is not otherwise visible anywhere.
		w.logger.Warn("webhook reaper requeued abandoned deliveries", "count", n)
	}
}

func (w *Worker) maybePurge(ctx context.Context) {
	now := w.now()
	if !w.lastPurge.IsZero() && now.Sub(w.lastPurge) < purgeInterval {
		return
	}
	w.lastPurge = now

	n, err := w.queue.Purge(ctx, now.AddDate(0, 0, -w.cfg.RetentionDays))
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Error("webhook retention purge failed", "err", err)
		}
		return
	}
	if n > 0 {
		w.logger.Info("webhook retention purge", "deleted", n, "retention_days", w.cfg.RetentionDays)
	}
}

func (w *Worker) releaseRemaining(rest []*ClaimedDelivery) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 5*time.Second)
	defer cancel()

	for _, cd := range rest {
		if err := w.queue.Release(ctx, cd.ID); err != nil {
			// Nothing more to do — the reaper will get it once its
			// threshold passes. Logged so the delay has an explanation.
			w.logger.Error("webhook release on shutdown failed", "delivery_id", cd.ID, "err", err)
		}
	}
	w.logger.Info("webhook worker released unsent deliveries on shutdown", "count", len(rest))
}

// deliver performs one attempt and records its outcome.
//
// Nothing here logs the payload or the secret (Rule #26): the payload is a
// full lead, and the secret is the credential the whole scheme rests on.
// Log lines carry ids and status codes only.
func (w *Worker) deliver(ctx context.Context, cd *ClaimedDelivery) {
	secret, err := w.crypter.Decrypt(cd.EndpointSecretCiphertext)
	if err != nil {
		// The encryption key was rotated, or the column is corrupt. Either
		// way this delivery can never be signed, and no number of retries
		// changes that — so it fails permanently rather than occupying the
		// queue for eight hours on its way to the same place.
		// (Key rotation has no staged path yet: docs/issues/101.)
		w.logger.Error("webhook signing secret cannot be decrypted",
			"delivery_id", cd.ID, "endpoint_id", cd.EndpointID, "organization_id", cd.OrganizationID)
		w.finish(ctx, cd, StatusFailed, cd.Attempt, nil, "signing secret cannot be decrypted")
		return
	}

	body := injectDeliveryID(cd.Payload, cd.ID)
	// Sign the bytes that go on the wire, using the shared Sign — never a
	// header assembled here (the wire contract in docs/issues/101).
	signature := Sign(string(secret), w.now(), body)

	reqCtx, cancel := context.WithTimeout(ctx, w.cfg.DeliveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cd.EndpointURL, bytes.NewReader(body))
	if err != nil {
		// A URL that passed validation at save time but will not parse
		// into a request now is broken in a way retrying cannot fix.
		w.finish(ctx, cd, StatusFailed, cd.Attempt, nil, "request could not be built")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Jualin-Webhook/1")
	req.Header.Set(SignatureHeader, signature)

	resp, err := w.client.Do(req)
	if err != nil {
		// Transport-level: DNS failure, connection refused, timeout, or a
		// target that resolved into a denied range at send time. All
		// transient in principle, so all retried — including the denied
		// range, since that may be a temporary DNS answer.
		w.retryOrFail(ctx, cd, nil, transportError(err))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	// Read a bounded amount and drop it. The receiver's body is never
	// stored — it may hold their own customers' data (TD §4.3).
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimit))

	status := resp.StatusCode
	switch {
	case status >= 200 && status < 300:
		w.finish(ctx, cd, StatusSucceeded, cd.Attempt, &status, "")
	case status == http.StatusTooManyRequests:
		// The receiver asked us to slow down; that is a request to come
		// back, not a rejection.
		w.retryOrFail(ctx, cd, &status, fmt.Sprintf("HTTP %d", status))
	case status >= 300 && status < 400:
		// Never followed (§3.3). Repeating it produces the same redirect,
		// so it is permanent, not transient.
		w.finish(ctx, cd, StatusFailed, cd.Attempt, &status, "redirect not followed")
	case status >= 400 && status < 500:
		// "Your request is wrong" does not become right by repetition (D5).
		w.finish(ctx, cd, StatusFailed, cd.Attempt, &status, fmt.Sprintf("HTTP %d", status))
	default:
		w.retryOrFail(ctx, cd, &status, fmt.Sprintf("HTTP %d", status))
	}
}

// retryOrFail schedules the next attempt, or gives up if this was the
// last one. attempt counts RETRIES, so cd.Attempt+1 is the retry this
// failure earns and MaxAttempts is the highest one allowed.
func (w *Worker) retryOrFail(ctx context.Context, cd *ClaimedDelivery, status *int, reason string) {
	next := cd.Attempt + 1
	if next > w.cfg.MaxAttempts {
		w.logger.Warn("webhook delivery failed permanently",
			"delivery_id", cd.ID, "endpoint_id", cd.EndpointID, "retries", cd.Attempt, "reason", reason)
		w.finish(ctx, cd, StatusFailed, cd.Attempt, status, reason)
		return
	}

	at := w.now().Add(backoff(next))
	w.record(ctx, cd, DeliveryResult{
		Status:         StatusPending,
		Attempt:        next,
		NextAttemptAt:  at,
		ResponseStatus: status,
		ErrorText:      &reason,
	})
}

func (w *Worker) finish(ctx context.Context, cd *ClaimedDelivery, status string, attempt int, responseStatus *int, reason string) {
	res := DeliveryResult{
		Status:         status,
		Attempt:        attempt,
		NextAttemptAt:  w.now(),
		ResponseStatus: responseStatus,
	}
	if reason != "" {
		res.ErrorText = &reason
	}
	w.record(ctx, cd, res)
}

func (w *Worker) record(ctx context.Context, cd *ClaimedDelivery, res DeliveryResult) {
	// A cancelled parent context must not stop the result from being
	// written: the HTTP call already happened, and losing its outcome
	// would send it a second time after restart for no reason.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	err := w.queue.MarkResult(writeCtx, cd.ID, res)
	switch {
	case err == nil:
	case errors.Is(err, ErrDeliveryNotClaimed):
		// The reaper took it back mid-flight. At-least-once in action
		// (TD §4.2) — the receiver may see this delivery twice, which is
		// why every payload carries a stable delivery_id.
		w.logger.Warn("webhook delivery result discarded, row was requeued mid-flight",
			"delivery_id", cd.ID)
	default:
		w.logger.Error("webhook could not record delivery result", "delivery_id", cd.ID, "err", err)
	}
}

// transportError produces the short string stored in the error column.
// url.Error's own message embeds the full target URL, which is fine to
// log but pointless to store per attempt; the endpoint is already
// identified by endpoint_id.
func transportError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return "request timed out"
		}
		return "transport error: " + urlErr.Err.Error()
	}
	return "transport error: " + err.Error()
}
