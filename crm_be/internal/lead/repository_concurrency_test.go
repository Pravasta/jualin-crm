package lead_test

// These three tests are the actual point of issue #19 — lead_number
// allocation and optimistic locking both fail SILENTLY under
// concurrency. A sequential test (create leads one at a time, or run
// updates one after another) stays green even with the row lock or the
// version check removed entirely, because there's never two writers
// racing on the same row. Only genuine concurrent goroutines prove
// anything here — see docs/phases/02-crm-core/td.md §16.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const concurrentWriters = 20

// TestConcurrency_LeadNumber_NoGapsNoDuplicates proves TD §3: N
// goroutines racing Create for the same organization must land on
// exactly {1..N}, no gaps, no duplicates — the row lock the
// UPDATE ... RETURNING statement takes on organizations.next_lead_number
// serializes concurrent allocation correctly.
//
// This test alone does NOT prove Create needs db.InTx — a single
// UPDATE...RETURNING is atomic whether or not it's wrapped in an
// explicit transaction, and this test never makes the following INSERT
// fail. What db.InTx actually buys — rollback undoing the allocation
// when the INSERT fails, so a rejected Create doesn't burn a number — is
// proven separately by TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber
// in repository_test.go. Checked by hand while writing this: temporarily
// calling Create straight against the pool (no db.InTx at all) still
// passes this exact test, which is why that second, more targeted test
// exists.
func TestConcurrency_LeadNumber_NoGapsNoDuplicates(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	org := seedOrganization(t, ctx, pool)

	numbers := createConcurrently(t, ctx, pool, org, concurrentWriters)

	assertExactSequence(t, numbers, concurrentWriters)
}

// TestConcurrency_LeadNumber_IndependentPerOrganization proves
// allocation serializes per organization, not globally — both
// organizations independently land on {1..N/2}.
func TestConcurrency_LeadNumber_IndependentPerOrganization(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)

	const perOrg = concurrentWriters / 2
	var wg sync.WaitGroup
	numbersA := make([]int, perOrg)
	numbersB := make([]int, perOrg)

	wg.Add(2)
	go func() { defer wg.Done(); copy(numbersA, createConcurrently(t, ctx, pool, orgA, perOrg)) }()
	go func() { defer wg.Done(); copy(numbersB, createConcurrently(t, ctx, pool, orgB, perOrg)) }()
	wg.Wait()

	assertExactSequence(t, numbersA, perOrg)
	assertExactSequence(t, numbersB, perOrg)
}

// TestConcurrency_VersionConflict_ExactlyOneWins proves TD §4: M
// concurrent Update calls starting from the same version must produce
// exactly one success — never zero, never more than one.
func TestConcurrency_VersionConflict_ExactlyOneWins(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)
	org := seedOrganization(t, ctx, pool)

	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, minimalInput("Contested Lead"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	const writers = concurrentWriters
	var wg sync.WaitGroup
	var successes, conflicts int
	var mu sync.Mutex

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := "Writer Name"
			_, err := repo.Update(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, created.ID, created.Version, lead.UpdateInput{Name: &name})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successes++
			case errors.Is(err, lead.ErrVersionConflict):
				conflicts++
			default:
				t.Errorf("writer %d: unexpected error: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("expected exactly 1 successful update out of %d concurrent writers, got %d", writers, successes)
	}
	if conflicts != writers-1 {
		t.Errorf("expected %d version conflicts, got %d", writers-1, conflicts)
	}
}

// createConcurrently fires n goroutines, each in its own db.InTx, and
// returns the resulting lead_numbers.
func createConcurrently(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, n int) []int {
	t.Helper()

	numbers := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
				repo := lead.New(tx)
				created, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, minimalInput("Concurrent Lead"))
				if err != nil {
					return err
				}
				numbers[idx] = created.LeadNumber
				return nil
			})
			if err != nil {
				t.Errorf("goroutine %d: create failed: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
	return numbers
}

func assertExactSequence(t *testing.T, numbers []int, n int) {
	t.Helper()
	seen := make(map[int]bool, n)
	for _, num := range numbers {
		if seen[num] {
			t.Errorf("duplicate lead_number %d", num)
		}
		seen[num] = true
	}
	for i := 1; i <= n; i++ {
		if !seen[i] {
			t.Errorf("missing lead_number %d — gap in sequence", i)
		}
	}
}
