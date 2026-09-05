package lead_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func TestRepository_Create_FirstLeadInOrgGetsNumberOne(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)

	org := seedOrganization(t, ctx, pool)

	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, minimalInput("Budi"))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.LeadNumber != 1 {
		t.Errorf("expected lead_number 1 for a fresh organization, got %d", created.LeadNumber)
	}
	if created.Version != 1 {
		t.Errorf("expected initial version 1, got %d", created.Version)
	}
	if created.Status != "new" {
		t.Errorf("expected default status 'new', got %q", created.Status)
	}
}

// TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber is the actual
// proof that Create must be called inside db.InTx (TD §3) — see the
// doc comment on Create and on the concurrency tests in
// repository_concurrency_test.go for why the concurrency tests alone
// don't establish this: they never make the INSERT fail, so they'd
// pass identically even if Create were called against a bare pool.
func TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)
	org := seedOrganization(t, ctx, pool)

	first, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, minimalInput("First"))
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}
	if first.LeadNumber != 1 {
		t.Fatalf("expected the seed lead to get number 1, got %d", first.LeadNumber)
	}

	// This Create is guaranteed to fail at the INSERT step: the FK on
	// assigned_to_membership_id points at a membership that doesn't
	// exist. Run inside db.InTx, so the whole attempt — including the
	// next_lead_number allocation that happened first — rolls back.
	bogusMembership := uuid.Must(uuid.NewV7())
	failingInput := minimalInput("Will Fail")
	failingInput.AssignedToMembershipID = &bogusMembership

	txErr := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := lead.New(tx).Create(ctx, tenant.Context{OrganizationID: org}, failingInput)
		return err
	})
	if txErr == nil {
		t.Fatal("expected the create to fail on the FK violation")
	}

	// If the allocation had NOT been rolled back with it, this next
	// create would land on number 3 (1 already burned by the failed
	// attempt). It must land on 2.
	second, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, minimalInput("Second"))
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if second.LeadNumber != 2 {
		t.Errorf("expected lead_number 2 (rolled-back attempt should leave no gap), got %d", second.LeadNumber)
	}
}

func TestRepository_FindByID_CrossTenant_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)

	created, err := repo.Create(ctx, tenant.Context{OrganizationID: orgA}, minimalInput("Org A Lead"))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = repo.FindByID(ctx, tenant.Context{OrganizationID: orgB, Role: tenant.RoleOwner}, created.ID)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a lead in a different organization, got: %v", err)
	}
}

// TestRepository_FindByID_Employee_OnlySeesOwnAssignedLead is the PRD's
// core requirement for this issue (TD §9) — enforced in the repository,
// not left to a future usecase to remember.
func TestRepository_FindByID_Employee_OnlySeesOwnAssignedLead(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)

	org := seedOrganization(t, ctx, pool)
	employeeA := seedMembership(t, ctx, pool, org, "employee-a@example.com", tenant.RoleEmployee)
	employeeB := seedMembership(t, ctx, pool, org, "employee-b@example.com", tenant.RoleEmployee)

	in := minimalInput("Assigned to A")
	in.AssignedToMembershipID = &employeeA
	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, in)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Employee B must not see it.
	_, err = repo.FindByID(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &employeeB}, created.ID)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for an employee reading someone else's lead, got: %v", err)
	}

	// Employee A (the assignee) must see it.
	found, err := repo.FindByID(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &employeeA}, created.ID)
	if err != nil {
		t.Fatalf("expected the assignee to read their own lead, got: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected the same lead back, got a different id")
	}

	// Owner sees everything regardless of assignment.
	ownerID := seedMembership(t, ctx, pool, org, "owner@example.com", tenant.RoleOwner)
	if _, err := repo.FindByID(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner, MembershipID: &ownerID}, created.ID); err != nil {
		t.Errorf("expected owner to read any lead in their organization, got: %v", err)
	}
}

func TestRepository_Update_CorrectVersion_Succeeds(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)

	org := seedOrganization(t, ctx, pool)
	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, minimalInput("Original Name"))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	newName := "Updated Name"
	updated, err := repo.Update(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, created.ID, created.Version, lead.UpdateInput{Name: &newName})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected name %q, got %q", newName, updated.Name)
	}
	if updated.Version != created.Version+1 {
		t.Errorf("expected version to increment from %d to %d, got %d", created.Version, created.Version+1, updated.Version)
	}
}

// TestRepository_Update_StaleVersion_ReturnsConflictWithCurrentState is
// TD §4's contract: the caller gets the current row back alongside
// ErrVersionConflict, without a second query.
func TestRepository_Update_StaleVersion_ReturnsConflictWithCurrentState(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)

	org := seedOrganization(t, ctx, pool)
	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, minimalInput("Name"))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	staleVersion := created.Version - 1 // never a real version — always stale
	newName := "Should Not Apply"
	current, err := repo.Update(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, created.ID, staleVersion, lead.UpdateInput{Name: &newName})
	if !errors.Is(err, lead.ErrVersionConflict) {
		t.Fatalf("expected lead.ErrVersionConflict, got: %v", err)
	}
	if current == nil {
		t.Fatal("expected the current lead state to be returned alongside the conflict")
	}
	if current.Name == newName {
		t.Error("the stale update must not have applied")
	}
	if current.Version != created.Version {
		t.Errorf("expected returned version to be the actual current version %d, got %d", created.Version, current.Version)
	}
}

func TestRepository_Update_MissingLead_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)

	org := seedOrganization(t, ctx, pool)
	newName := "Doesn't Matter"
	_, err := repo.Update(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, uuid.Must(uuid.NewV7()), 1, lead.UpdateInput{Name: &newName})
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a nonexistent lead, got: %v", err)
	}
}

func minimalInput(name string) lead.CreateInput {
	return lead.CreateInput{Name: name, Source: "manual"}
}

func seedOrganization(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Test Org')`, id); err != nil {
		t.Fatalf("failed to seed organization: %v", err)
	}
	return id
}

func seedMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, email string, role tenant.Role) uuid.UUID {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Test User')`, userID, email); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	membershipID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, $4)`, membershipID, org, userID, role); err != nil {
		t.Fatalf("failed to seed membership: %v", err)
	}
	return membershipID
}

// --- CountCreatedThisMonth (Phase 8.5 #122) ---

// TestRepository_CountCreatedThisMonth_IncludesSoftDeleted is the
// property that lets a COUNT stand in for a usage_counters table
// (08.5 prd D1): deleting a lead must NOT hand the quota back.
// Without it there is a trivial abuse path — create up to the limit,
// delete, repeat forever.
func TestRepository_CountCreatedThisMonth_IncludesSoftDeleted(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)
	org := seedOrganization(t, ctx, pool)
	tc := tenant.Context{OrganizationID: org}

	kept, err := repo.Create(ctx, tc, minimalInput("Tetap"))
	if err != nil {
		t.Fatalf("create kept: %v", err)
	}
	deleted, err := repo.Create(ctx, tc, minimalInput("Dihapus"))
	if err != nil {
		t.Fatalf("create deleted: %v", err)
	}
	if err := repo.Delete(ctx, tc, deleted.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	n, err := repo.CountCreatedThisMonth(ctx, tc)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 (the deleted lead still consumes quota), got %d", n)
	}

	// Sanity: the deleted one really is gone from normal reads, so the
	// count above is not just "delete did nothing".
	if _, err := repo.FindByID(ctx, tc, deleted.ID); err == nil {
		t.Error("expected the soft-deleted lead to be invisible to FindByID")
	}
	_ = kept
}

func TestRepository_CountCreatedThisMonth_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)

	for i := 0; i < 3; i++ {
		if _, err := repo.Create(ctx, tenant.Context{OrganizationID: orgB}, minimalInput("Milik B")); err != nil {
			t.Fatalf("seed org B: %v", err)
		}
	}

	n, err := repo.CountCreatedThisMonth(ctx, tenant.Context{OrganizationID: orgA})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("expected org A to count 0 of org B's 3 leads, got %d", n)
	}
}

// TestRepository_CountCreatedThisMonth_ExcludesPreviousMonth proves the
// window is the current month rather than all time — and that it is cut
// in the ORGANIZATION'S timezone (Rule #13), not UTC. The lead below is
// backdated to the last instant of the previous month in Asia/Jakarta;
// counting in UTC would place it 7 hours earlier, which is still the
// previous month, so this case is only sharp about the month boundary.
// The timezone itself is asserted separately below.
func TestRepository_CountCreatedThisMonth_ExcludesPreviousMonth(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)
	org := seedOrganization(t, ctx, pool)
	tc := tenant.Context{OrganizationID: org}

	current, err := repo.Create(ctx, tc, minimalInput("Bulan ini"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	old, err := repo.Create(ctx, tc, minimalInput("Bulan lalu"))
	if err != nil {
		t.Fatalf("create old: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE leads SET created_at = date_trunc('month', now()) - interval '1 day' WHERE id = $1`, old.ID,
	); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	n, err := repo.CountCreatedThisMonth(ctx, tc)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected only this month's lead to count, got %d", n)
	}
	_ = current
}

// TestRepository_CountCreatedThisMonth_UsesOrganizationTimezone is the
// case UTC would get wrong. A lead created at 00:30 on the 1st in
// Asia/Jakarta is 17:30 on the LAST day of the previous month in UTC —
// so an implementation that truncates in UTC drops it from the count,
// and the customer's quota looks like it reset late.
func TestRepository_CountCreatedThisMonth_UsesOrganizationTimezone(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := lead.New(pool)
	org := seedOrganization(t, ctx, pool)
	tc := tenant.Context{OrganizationID: org}

	created, err := repo.Create(ctx, tc, minimalInput("Dini hari tanggal 1"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 00:30 on the 1st, Asia/Jakarta — the seeded organization's default
	// timezone (migration 0002).
	if _, err := pool.Exec(ctx,
		`UPDATE leads
		    SET created_at = (date_trunc('month', now() AT TIME ZONE 'Asia/Jakarta') + interval '30 minutes') AT TIME ZONE 'Asia/Jakarta'
		  WHERE id = $1`, created.ID,
	); err != nil {
		t.Fatalf("backdate to 00:30 local: %v", err)
	}

	n, err := repo.CountCreatedThisMonth(ctx, tc)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected the 00:30-on-the-1st lead to count in its own timezone, got %d — counting in UTC would place it in the previous month", n)
	}
}
