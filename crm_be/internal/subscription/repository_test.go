package subscription_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
)

func TestRepository_FindActiveByOrg_ReturnsTheRightRow(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := subscription.New(pool)

	org := seedOrganization(t, ctx, pool)
	seedSubscription(t, ctx, pool, org, "free", "active")

	sub, err := repo.FindActiveByOrg(ctx, tenant.Context{OrganizationID: org})
	if err != nil {
		t.Fatalf("find active by org: %v", err)
	}
	if sub.PlanCode != "free" || sub.Status != "active" {
		t.Errorf("got plan_code=%q status=%q, want free/active", sub.PlanCode, sub.Status)
	}
	if sub.OrganizationID != org {
		t.Errorf("got organization_id=%s, want %s", sub.OrganizationID, org)
	}
}

// TestRepository_FindActiveByOrg_TenantIsolation is the required
// isolation test for a feature touching a tenant boundary (CLAUDE.md
// Aturan #7): org A must never see org B's subscription, even indirectly
// through a query that only filters on status.
func TestRepository_FindActiveByOrg_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := subscription.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	seedSubscription(t, ctx, pool, orgB, "free", "active")
	// orgA deliberately has NO subscription row.

	_, err := repo.FindActiveByOrg(ctx, tenant.Context{OrganizationID: orgA})
	if !errors.Is(err, subscription.ErrNoActiveSubscription) {
		t.Fatalf("expected ErrNoActiveSubscription for org with no row of its own, got %v", err)
	}
}

func TestRepository_FindActiveByOrg_NonActiveStatus_NotReturned(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := subscription.New(pool)

	org := seedOrganization(t, ctx, pool)
	seedSubscription(t, ctx, pool, org, "free", "past_due")

	_, err := repo.FindActiveByOrg(ctx, tenant.Context{OrganizationID: org})
	if !errors.Is(err, subscription.ErrNoActiveSubscription) {
		t.Fatalf("expected ErrNoActiveSubscription for a non-active row, got %v", err)
	}
}

// --- ChangePlan (Phase 8.5 #124) ---

func TestRepository_ChangePlan_UpdatesPlanCode(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := subscription.New(pool)

	org := seedOrganization(t, ctx, pool)
	seedSubscription(t, ctx, pool, org, "free", "active")

	if err := repo.ChangePlan(ctx, tenant.Context{OrganizationID: org}, "pro"); err != nil {
		t.Fatalf("change plan: %v", err)
	}

	sub, err := repo.FindActiveByOrg(ctx, tenant.Context{OrganizationID: org})
	if err != nil {
		t.Fatalf("find active by org: %v", err)
	}
	if sub.PlanCode != "pro" {
		t.Errorf("expected plan_code %q, got %q", "pro", sub.PlanCode)
	}
}

// TestRepository_ChangePlan_TargetsRowRegardlessOfStatus proves an
// admin can fix a past_due organization's plan — ChangePlan targets by
// organization_id alone, not status='active' (its own doc comment).
func TestRepository_ChangePlan_TargetsRowRegardlessOfStatus(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := subscription.New(pool)

	org := seedOrganization(t, ctx, pool)
	seedSubscription(t, ctx, pool, org, "free", "past_due")

	if err := repo.ChangePlan(ctx, tenant.Context{OrganizationID: org}, "pro"); err != nil {
		t.Fatalf("change plan: %v", err)
	}

	var planCode string
	if err := pool.QueryRow(ctx, `SELECT plan_code FROM subscriptions WHERE organization_id = $1`, org).Scan(&planCode); err != nil {
		t.Fatalf("query: %v", err)
	}
	if planCode != "pro" {
		t.Errorf("expected plan_code %q, got %q", "pro", planCode)
	}
}

func TestRepository_ChangePlan_NoSubscriptionRow_ReturnsErrNoActiveSubscription(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := subscription.New(pool)

	org := seedOrganization(t, ctx, pool) // no subscription row seeded

	err := repo.ChangePlan(ctx, tenant.Context{OrganizationID: org}, "pro")
	if !errors.Is(err, subscription.ErrNoActiveSubscription) {
		t.Fatalf("expected ErrNoActiveSubscription, got %v", err)
	}
}

func TestRepository_ChangePlan_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := subscription.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	seedSubscription(t, ctx, pool, orgA, "free", "active")
	seedSubscription(t, ctx, pool, orgB, "free", "active")

	if err := repo.ChangePlan(ctx, tenant.Context{OrganizationID: orgA}, "pro"); err != nil {
		t.Fatalf("change plan: %v", err)
	}

	var orgBPlanCode string
	if err := pool.QueryRow(ctx, `SELECT plan_code FROM subscriptions WHERE organization_id = $1`, orgB).Scan(&orgBPlanCode); err != nil {
		t.Fatalf("query org B: %v", err)
	}
	if orgBPlanCode != "free" {
		t.Errorf("expected org B untouched (still free), got %q", orgBPlanCode)
	}
}

func seedOrganization(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Test Org')`, id); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	return id
}

func seedSubscription(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, planCode, status string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions (id, organization_id, plan_code, status) VALUES ($1, $2, $3, $4)`, id, org, planCode, status); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return id
}
