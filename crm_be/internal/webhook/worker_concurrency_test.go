package webhook_test

// Acceptance criterion #12: two instances must never deliver the same row
// twice.
//
// Checked by hand while writing these, and worth recording because it
// contradicts the obvious reading of TD §4.1: deleting SKIP LOCKED from
// the claim query does NOT make the exactly-once test below fail. Without
// it the competing statements BLOCK instead of skipping, and when the lock
// releases PostgreSQL re-evaluates the subquery against the updated row —
// which is now 'delivering' and no longer matches WHERE status = 'pending'.
//
// So the two clauses do different jobs, and only one of them is about
// correctness:
//
//   - WHERE status = 'pending' + row locking gives EXACTLY-ONCE.
//   - SKIP LOCKED gives LIVENESS: claimers never wait on each other.
//
// Both are tested, separately, because a test that cannot fail proves
// nothing — TestConcurrency_ClaimDue_EachRowClaimedExactlyOnce for the
// first, TestConcurrency_ClaimDue_DoesNotBlockOnALockedRow for the second.
// Same discipline as internal/lead's lead_number tests (#19), which make
// the same distinction between what a concurrency test does and does not
// actually prove.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

const (
	concurrentClaimers = 8
	rowsToClaim        = 40
)

// TestConcurrency_ClaimDue_EachRowClaimedExactlyOnce fires N claimers at
// one queue simultaneously and asserts the union of what they took is
// exactly the set of due rows, with no id appearing twice.
//
// Repeated: a race that reproduces one run in three is worse than one that
// never reproduces, because a single green run reads as proof. Looping
// inside the test (rather than relying on -count) means the guarantee is
// exercised on every ordinary `go test` run too.
func TestConcurrency_ClaimDue_EachRowClaimedExactlyOnce(t *testing.T) {
	for round := 0; round < 3; round++ {
		ctx := context.Background()
		pool := dbtest.NewPool(t)
		repo := webhook.New(pool)
		drepo := webhook.NewDeliveryRepository(pool)

		org := seedOrg(t, ctx, pool)
		e := newTestEndpoint()
		if err := repo.Create(ctx, tctx(org), e); err != nil {
			t.Fatalf("round %d: create endpoint: %v", round, err)
		}

		want := make(map[uuid.UUID]bool, rowsToClaim)
		for i := 0; i < rowsToClaim; i++ {
			id := seedDelivery(t, ctx, pool, org, e.ID, "pending", time.Now().Add(-time.Minute))
			want[id] = true
		}

		// All claimers wait on one gate so they hit the queue together
		// rather than drifting into a de facto sequence.
		var gate sync.WaitGroup
		gate.Add(1)
		var done sync.WaitGroup
		results := make([][]*webhook.ClaimedDelivery, concurrentClaimers)
		errs := make([]error, concurrentClaimers)

		for i := 0; i < concurrentClaimers; i++ {
			done.Add(1)
			go func(idx int) {
				defer done.Done()
				gate.Wait()
				// Each claimer asks for more than its fair share, so they
				// genuinely compete for the same rows instead of each
				// finding a tidy disjoint slice.
				results[idx], errs[idx] = drepo.ClaimDue(ctx, rowsToClaim)
			}(i)
		}
		gate.Done()
		done.Wait()

		seen := make(map[uuid.UUID]int, rowsToClaim)
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d: claimer %d: %v", round, i, err)
			}
			for _, cd := range results[i] {
				seen[cd.ID]++
			}
		}

		for id, count := range seen {
			if count != 1 {
				t.Fatalf("round %d: delivery %s was claimed %d times — SKIP LOCKED is not holding, and this row would be delivered twice", round, id, count)
			}
			if !want[id] {
				t.Fatalf("round %d: claimed a row that was not due: %s", round, id)
			}
		}
		if len(seen) != rowsToClaim {
			t.Fatalf("round %d: %d of %d due rows were claimed; the rest were lost", round, len(seen), rowsToClaim)
		}

		// Every claimed row must actually be marked delivering in the
		// database — the claim is not just a read.
		var stillPending int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE organization_id = $1 AND status <> 'delivering'`, org).Scan(&stillPending); err != nil {
			t.Fatalf("round %d: count: %v", round, err)
		}
		if stillPending != 0 {
			t.Fatalf("round %d: %d claimed rows are not marked delivering", round, stillPending)
		}
	}
}

// TestConcurrency_ClaimDue_RespectsLimit guards the other half: SKIP
// LOCKED must not be achieved by simply handing everyone everything.
func TestConcurrency_ClaimDue_RespectsLimit(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	org := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tctx(org), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	for i := 0; i < 10; i++ {
		seedDelivery(t, ctx, pool, org, e.ID, "pending", time.Now().Add(-time.Minute))
	}

	claimed, err := drepo.ClaimDue(ctx, 3)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 3 {
		t.Fatalf("claimed %d rows with limit 3", len(claimed))
	}
}

// TestClaimDue_CarriesEndpointURLAndSecret proves the JOIN actually
// populates what the worker sends with. Without it the worker would have
// to look the endpoint up separately, leaving a window in which it is
// edited between claim and send.
func TestClaimDue_CarriesEndpointURLAndSecret(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	org := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	e.URL = "https://receiver.example.com/very-specific-path"
	e.SecretCiphertext = []byte("sealed-bytes-for-this-endpoint")
	if err := repo.Create(ctx, tctx(org), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	seedDelivery(t, ctx, pool, org, e.ID, "pending", time.Now().Add(-time.Minute))

	claimed, err := drepo.ClaimDue(ctx, 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("claimed %d rows, want 1", len(claimed))
	}
	if claimed[0].EndpointURL != e.URL {
		t.Errorf("EndpointURL = %q, want %q", claimed[0].EndpointURL, e.URL)
	}
	if string(claimed[0].EndpointSecretCiphertext) != string(e.SecretCiphertext) {
		t.Errorf("EndpointSecretCiphertext = %q, want %q", claimed[0].EndpointSecretCiphertext, e.SecretCiphertext)
	}
}

// TestConcurrency_ClaimDue_DoesNotBlockOnALockedRow is the test that
// actually observes SKIP LOCKED, by making its absence the difference
// between returning and hanging.
//
// A separate transaction locks one due row and holds it. ClaimDue is then
// called with a short deadline. With SKIP LOCKED it steps over the locked
// row and returns the others immediately; without it, the statement waits
// for a lock that is not released until after the deadline, and the call
// fails instead of returning.
//
// This matters beyond tidiness: one instance stuck mid-claim would stall
// every other instance's claim behind it, turning a queue meant to drain
// in parallel into a serial one.
func TestConcurrency_ClaimDue_DoesNotBlockOnALockedRow(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	org := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tctx(org), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	locked := seedDelivery(t, ctx, pool, org, e.ID, "pending", time.Now().Add(-2*time.Minute))
	free := seedDelivery(t, ctx, pool, org, e.ID, "pending", time.Now().Add(-time.Minute))

	// Hold a row lock the way a peer instance mid-claim would.
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, `SELECT id FROM webhook_deliveries WHERE id = $1 FOR UPDATE`, locked); err != nil {
		t.Fatalf("lock row: %v", err)
	}

	claimCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	claimed, err := drepo.ClaimDue(claimCtx, 10)
	if err != nil {
		t.Fatalf("ClaimDue blocked behind a locked row instead of skipping it: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != free {
		t.Fatalf("expected exactly the unlocked row, got %d rows", len(claimed))
	}
}
