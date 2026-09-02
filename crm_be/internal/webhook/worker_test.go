package webhook_test

// The worker against a real HTTP server and real Postgres (Phase 7 #102,
// TD §4.3). Every case here is about what the OUTSIDE world does to us —
// status codes, timeouts, redirects — so the receiver is an
// httptest.Server rather than a stubbed RoundTripper: a stub would only
// replay our own assumptions about net/http back at us.

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/safedial"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

// fixedNow pins the worker's clock so next_attempt_at can be asserted to
// the second instead of "roughly an hour from now".
var fixedNow = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

type workerFixture struct {
	pool     *pgxpool.Pool
	worker   *webhook.Worker
	org      uuid.UUID
	endpoint uuid.UUID
	secret   string
}

// newWorkerFixture wires the real worker against a real database and the
// given receiver URL, with allowPrivate on (the receiver is on loopback).
func newWorkerFixture(t *testing.T, receiverURL string, timeout time.Duration, maxRetries int) *workerFixture {
	t.Helper()
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	org := seedOrg(t, ctx, pool)

	// A real sealed secret, so the worker's decrypt path is exercised
	// rather than bypassed.
	const raw = "whsec_worker-test-secret-value-0123456789ab"
	sealed, err := testCrypter().Encrypt([]byte(raw))
	if err != nil {
		t.Fatalf("seal secret: %v", err)
	}

	e := newTestEndpoint()
	e.URL = receiverURL
	e.SecretCiphertext = sealed
	if err := webhook.New(pool).Create(ctx, tctx(org), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	w := webhook.NewWorker(
		webhook.NewDeliveryRepository(pool),
		safedial.NewValidator(true).HTTPClient(timeout),
		testCrypter(),
		webhook.WorkerConfig{
			Interval:        time.Hour, // irrelevant: tests drive ticks directly
			Batch:           10,
			DeliveryTimeout: timeout,
			MaxAttempts:     maxRetries,
			RetentionDays:   30,
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	webhook.SetWorkerClockForTest(w, func() time.Time { return fixedNow })

	return &workerFixture{pool: pool, worker: w, org: org, endpoint: e.ID, secret: raw}
}

// runOnce drives exactly one tick, the way Run would, without the ticker.
func (f *workerFixture) runOnce(t *testing.T) {
	t.Helper()
	webhook.RunWorkerTickForTest(f.worker, context.Background())
}

func (f *workerFixture) enqueue(t *testing.T, attempt int) uuid.UUID {
	t.Helper()
	id := seedDelivery(t, context.Background(), f.pool, f.org, f.endpoint, "pending", fixedNow.Add(-time.Minute))
	if attempt > 0 {
		if _, err := f.pool.Exec(context.Background(), `UPDATE webhook_deliveries SET attempt = $2 WHERE id = $1`, id, attempt); err != nil {
			t.Fatalf("set attempt: %v", err)
		}
	}
	return id
}

type deliveryRow struct {
	status         string
	attempt        int
	nextAttemptAt  time.Time
	responseStatus *int
	errText        *string
}

func (f *workerFixture) read(t *testing.T, id uuid.UUID) deliveryRow {
	t.Helper()
	var r deliveryRow
	err := f.pool.QueryRow(context.Background(),
		`SELECT status, attempt, next_attempt_at, response_status, error FROM webhook_deliveries WHERE id = $1`, id).
		Scan(&r.status, &r.attempt, &r.nextAttemptAt, &r.responseStatus, &r.errText)
	if err != nil {
		t.Fatalf("read delivery: %v", err)
	}
	return r
}

// TestWorker_StatusMapping walks TD §4.3's table end to end. Each case is
// one receiver answering one way, and one assertion about what the row
// looks like afterwards.
func TestWorker_StatusMapping(t *testing.T) {
	for _, tc := range []struct {
		name        string
		reply       int
		wantStatus  string
		wantAttempt int
	}{
		{"200 succeeds", http.StatusOK, webhook.StatusSucceeded, 0},
		{"201 succeeds", http.StatusCreated, webhook.StatusSucceeded, 0},
		{"204 succeeds", http.StatusNoContent, webhook.StatusSucceeded, 0},
		// "Your request is wrong" does not become right by repetition.
		{"400 fails permanently", http.StatusBadRequest, webhook.StatusFailed, 0},
		{"401 fails permanently", http.StatusUnauthorized, webhook.StatusFailed, 0},
		{"404 fails permanently", http.StatusNotFound, webhook.StatusFailed, 0},
		// The receiver asked us to come back later — that is not a rejection.
		{"429 is retried", http.StatusTooManyRequests, webhook.StatusPending, 1},
		{"500 is retried", http.StatusInternalServerError, webhook.StatusPending, 1},
		{"503 is retried", http.StatusServiceUnavailable, webhook.StatusPending, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.reply)
			}))
			defer srv.Close()

			f := newWorkerFixture(t, srv.URL, 2*time.Second, 5)
			id := f.enqueue(t, 0)
			f.runOnce(t)

			got := f.read(t, id)
			if got.status != tc.wantStatus {
				t.Errorf("status = %q, want %q", got.status, tc.wantStatus)
			}
			if got.attempt != tc.wantAttempt {
				t.Errorf("attempt = %d, want %d", got.attempt, tc.wantAttempt)
			}
			if got.responseStatus == nil || *got.responseStatus != tc.reply {
				t.Errorf("response_status = %v, want %d", got.responseStatus, tc.reply)
			}
		})
	}
}

// TestWorker_RedirectFailsPermanently is the SSRF bypass from TD §3.3 seen
// from the worker's side: the redirect is never followed, and the 3xx is
// terminal rather than retried (repeating it produces the same redirect).
func TestWorker_RedirectFailsPermanently(t *testing.T) {
	var secondRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hook" {
			secondRequest = true
		}
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL+"/hook", 2*time.Second, 5)
	id := f.enqueue(t, 0)
	f.runOnce(t)

	got := f.read(t, id)
	if got.status != webhook.StatusFailed {
		t.Errorf("status = %q, want failed — a redirect must not be retried", got.status)
	}
	if got.responseStatus == nil || *got.responseStatus != http.StatusFound {
		t.Errorf("response_status = %v, want 302", got.responseStatus)
	}
	if secondRequest {
		t.Error("the worker followed the redirect")
	}
}

// TestWorker_BackoffScheduleMatchesD5 asserts every delay in the table,
// not just that "some" delay was applied. The clock is fixed, so these are
// exact equalities.
func TestWorker_BackoffScheduleMatchesD5(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	for _, tc := range []struct {
		priorAttempts int
		wantDelay     time.Duration
	}{
		{0, 1 * time.Minute},
		{1, 5 * time.Minute},
		{2, 30 * time.Minute},
		{3, 2 * time.Hour},
		{4, 6 * time.Hour},
	} {
		f := newWorkerFixture(t, srv.URL, 2*time.Second, 5)
		id := f.enqueue(t, tc.priorAttempts)
		f.runOnce(t)

		got := f.read(t, id)
		if got.status != webhook.StatusPending {
			t.Fatalf("after %d prior retries: status = %q, want pending", tc.priorAttempts, got.status)
		}
		want := fixedNow.Add(tc.wantDelay)
		if !got.nextAttemptAt.UTC().Equal(want) {
			t.Errorf("after %d prior retries: next_attempt_at = %s, want %s (delay %s)",
				tc.priorAttempts, got.nextAttemptAt.UTC(), want, tc.wantDelay)
		}
	}
}

// TestWorker_LastRetryFailsPermanently is the boundary of D5: the retry
// numbered MaxAttempts is the last one, and its failure ends the delivery
// rather than scheduling a sixth.
func TestWorker_LastRetryFailsPermanently(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, 2*time.Second, 5)
	// Five retries already made: this send is the last one allowed.
	id := f.enqueue(t, 5)
	f.runOnce(t)

	got := f.read(t, id)
	if got.status != webhook.StatusFailed {
		t.Fatalf("status = %q, want failed after the final retry", got.status)
	}

	// And it must stay failed — a later tick must not pick it up again.
	f.runOnce(t)
	if again := f.read(t, id); again.status != webhook.StatusFailed {
		t.Errorf("a failed delivery was picked up again: status = %q", again.status)
	}
}

func TestWorker_TimeoutIsRetried(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, 200*time.Millisecond, 5)
	id := f.enqueue(t, 0)
	f.runOnce(t)

	got := f.read(t, id)
	if got.status != webhook.StatusPending {
		t.Errorf("status = %q, want pending — a timeout is transient", got.status)
	}
	if got.responseStatus != nil {
		t.Errorf("response_status = %v, want NULL — there was no response", got.responseStatus)
	}
	if got.errText == nil {
		t.Error("error column is empty; a timeout should record why")
	}
}

func TestWorker_ConnectionRefusedIsRetried(t *testing.T) {
	// A port nothing is listening on. Closing the server immediately gives
	// us an address guaranteed to refuse.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	f := newWorkerFixture(t, url, time.Second, 5)
	id := f.enqueue(t, 0)
	f.runOnce(t)

	if got := f.read(t, id); got.status != webhook.StatusPending {
		t.Errorf("status = %q, want pending — a refused connection is transient", got.status)
	}
}

// TestWorker_UndecryptableSecretFailsPermanently covers the key-rotation
// case. Retrying cannot make a secret decryptable, so the delivery must
// not spend eight hours of backoff arriving at the same place.
func TestWorker_UndecryptableSecretFailsPermanently(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, 2*time.Second, 5)
	// Corrupt the stored ciphertext, exactly as a rotated key would.
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE webhook_endpoints SET secret_ciphertext = $2 WHERE id = $1`,
		f.endpoint, []byte("not-a-valid-ciphertext")); err != nil {
		t.Fatalf("corrupt secret: %v", err)
	}
	id := f.enqueue(t, 0)
	f.runOnce(t)

	got := f.read(t, id)
	if got.status != webhook.StatusFailed {
		t.Fatalf("status = %q, want failed — an unsignable delivery can never succeed", got.status)
	}
	if got.errText == nil || *got.errText == "" {
		t.Error("error column should say why")
	}
}

// TestWorker_SignsWhatItSends is the wire contract from docs/issues/101,
// verified the way a receiver would: recompute the HMAC over the exact
// bytes that arrived and compare.
func TestWorker_SignsWhatItSends(t *testing.T) {
	type received struct {
		body      []byte
		signature string
	}
	got := make(chan received, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- received{body: b, signature: r.Header.Get(webhook.SignatureHeader)}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, 2*time.Second, 5)
	id := f.enqueue(t, 0)
	f.runOnce(t)

	select {
	case r := <-got:
		// Exactly what webhook.Sign produces over the received bytes.
		want := webhook.Sign(f.secret, fixedNow, r.body)
		if r.signature != want {
			t.Fatalf("signature does not verify against the delivered body\n got %q\nwant %q", r.signature, want)
		}
		// And the body must carry delivery_id — the receiver's only
		// deduplication handle under at-least-once.
		if !containsJSONString(r.body, "delivery_id", id.String()) {
			t.Errorf("delivered body is missing delivery_id %s: %s", id, r.body)
		}
	default:
		t.Fatal("receiver was never called")
	}
}

func containsJSONString(body []byte, key, value string) bool {
	return bytes.Contains(body, []byte(`"`+key+`":"`+value+`"`))
}

// TestWorker_RunStopsOnContextCancel is the "no dangling goroutine"
// acceptance criterion. Run must RETURN, not merely stop working — a
// goroutine that quietly stays parked is exactly what a deploy would then
// wait on forever, or worse, not wait on at all.
func TestWorker_RunStopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, time.Second, 5)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		f.worker.Run(ctx)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

// TestWorker_ReleasesUnsentRowsOnShutdown covers the shutdown path that
// goes beyond TD §13's minimum. Leaving claimed-but-unsent rows
// 'delivering' would be correct but would delay every one of them by the
// reaper's ten-minute threshold on every single deploy.
func TestWorker_ReleasesUnsentRowsOnShutdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, time.Second, 5)
	for i := 0; i < 5; i++ {
		f.enqueue(t, 0)
	}

	// A context cancelled before the tick begins: nothing is attempted, so
	// everything claimed must come straight back to the queue.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	webhook.RunWorkerTickForTest(f.worker, ctx)

	var delivering, pending int
	row := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FILTER (WHERE status = 'delivering'), count(*) FILTER (WHERE status = 'pending')
		 FROM webhook_deliveries WHERE organization_id = $1`, f.org)
	if err := row.Scan(&delivering, &pending); err != nil {
		t.Fatalf("count: %v", err)
	}
	if delivering != 0 {
		t.Errorf("%d rows left claimed after shutdown; they would wait out the reaper threshold", delivering)
	}
	if pending != 5 {
		t.Errorf("pending = %d, want all 5 back in the queue", pending)
	}
}

// TestWorker_ReaperRequeuesAbandonedRows proves the tick actually runs the
// reaper, not just that the repository method works (repository_test.go
// already covers that in isolation).
func TestWorker_ReaperRequeuesAbandonedRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, time.Second, 5)
	ctx := context.Background()

	id := f.enqueue(t, 0)
	// Simulate a process that claimed this row and died: 'delivering',
	// claimed longer ago than the threshold.
	if _, err := f.pool.Exec(ctx,
		`UPDATE webhook_deliveries SET status = 'delivering', delivering_since = $2 WHERE id = $1`,
		id, fixedNow.Add(-30*time.Minute)); err != nil {
		t.Fatalf("simulate abandoned row: %v", err)
	}

	f.runOnce(t)

	// Requeued by the reaper and then delivered by the same tick.
	if got := f.read(t, id); got.status != webhook.StatusSucceeded {
		t.Errorf("status = %q, want succeeded — the reaper should have requeued it and the tick delivered it", got.status)
	}
}

// TestWorker_ReaperLeavesFreshClaimsAlone is the other half, and the one
// that matters across instances: a row another process claimed seconds ago
// must not be stolen, or every delivery gets sent twice.
func TestWorker_ReaperLeavesFreshClaimsAlone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, time.Second, 5)
	ctx := context.Background()

	id := f.enqueue(t, 0)
	if _, err := f.pool.Exec(ctx,
		`UPDATE webhook_deliveries SET status = 'delivering', delivering_since = $2 WHERE id = $1`,
		id, fixedNow.Add(-1*time.Minute)); err != nil {
		t.Fatalf("simulate in-flight row: %v", err)
	}

	f.runOnce(t)

	if got := f.read(t, id); got.status != webhook.StatusDelivering {
		t.Errorf("status = %q, want delivering — a peer's in-flight delivery was stolen", got.status)
	}
}

// TestWorker_PurgeIsThrottled proves retention runs at most once an hour
// from inside the loop (TD §10), rather than on every tick.
func TestWorker_PurgeIsThrottled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := newWorkerFixture(t, srv.URL, time.Second, 5)
	ctx := context.Background()

	// Two terminal rows older than the retention window.
	old := fixedNow.AddDate(0, 0, -40)
	first := seedDeliveryAged(t, ctx, f.pool, f.org, f.endpoint, "succeeded", old)
	seedDeliveryAged(t, ctx, f.pool, f.org, f.endpoint, "failed", old)

	f.runOnce(t) // first tick purges
	var remaining int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE organization_id = $1`, f.org).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("%d aged rows survived the purge", remaining)
	}
	_ = first

	// A second aged row and another tick: the throttle must hold it, since
	// the clock has not advanced.
	seedDeliveryAged(t, ctx, f.pool, f.org, f.endpoint, "succeeded", old)
	f.runOnce(t)
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE organization_id = $1`, f.org).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Errorf("remaining = %d, want 1 — purge ran again within the hour", remaining)
	}
}
