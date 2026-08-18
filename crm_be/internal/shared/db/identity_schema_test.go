package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
)

// These tests exercise the guarantees migration 0002_identity makes at
// the database level — they are deliberately schema tests, not repository
// tests: the point is that these rules hold regardless of what Go code
// does, per docs/architecture/multi-tenancy.md lapis 2.

func TestCompositeFK_RejectsCrossTenantMembershipReference(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	orgA := insertOrganization(t, ctx, pool, "Org A")
	orgB := insertOrganization(t, ctx, pool, "Org B")
	userA := insertUser(t, ctx, pool, "a@example.com")
	membershipInA := insertMembership(t, ctx, pool, orgA, userA, "employee")

	// audit_logs.fk_audit_actor requires (actor_membership_id,
	// organization_id) to jointly match a row in memberships(id,
	// organization_id). membershipInA belongs to orgA, so referencing it
	// under orgB must be rejected by the database — not by application
	// code.
	_, err := pool.Exec(ctx, `
		INSERT INTO audit_logs (id, organization_id, actor_type, actor_membership_id, action)
		VALUES ($1, $2, 'user', $3, 'test.cross_tenant_attempt')`,
		uuid.Must(uuid.NewV7()), orgB, membershipInA,
	)
	if err == nil {
		t.Fatal("expected composite FK to reject a membership belonging to a different organization, but insert succeeded")
	}
}

func TestMembershipPartialUnique_RejectsDuplicateActive_AllowsAfterSoftDelete(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	org := insertOrganization(t, ctx, pool, "Org")
	u := insertUser(t, ctx, pool, "u@example.com")

	first := insertMembership(t, ctx, pool, org, u, "employee")

	// A second active membership for the same (org, user) must be
	// rejected by uq_memberships_org_user_active.
	_, err := pool.Exec(ctx, `
		INSERT INTO memberships (id, organization_id, user_id, role)
		VALUES ($1, $2, $3, 'manager')`,
		uuid.Must(uuid.NewV7()), org, u,
	)
	if err == nil {
		t.Fatal("expected a second active membership for the same (organization, user) to be rejected")
	}

	// Soft-deleting the first membership must free up the slot — the
	// partial unique index only covers deleted_at IS NULL rows.
	if _, err := pool.Exec(ctx, `UPDATE memberships SET deleted_at = now() WHERE id = $1`, first); err != nil {
		t.Fatalf("failed to soft-delete membership: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO memberships (id, organization_id, user_id, role)
		VALUES ($1, $2, $3, 'manager')`,
		uuid.Must(uuid.NewV7()), org, u,
	)
	if err != nil {
		t.Fatalf("expected a new active membership to be allowed once the old one is soft-deleted, got: %v", err)
	}
}

// TestMultiMembership_AllowedAcrossOrganizations guards ADR-007: a single
// user_id must be allowed to hold memberships in more than one
// organization. If anyone ever adds UNIQUE(user_id) to memberships, this
// test starts failing — that is exactly its job.
func TestMultiMembership_AllowedAcrossOrganizations(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	orgA := insertOrganization(t, ctx, pool, "Org A")
	orgB := insertOrganization(t, ctx, pool, "Org B")
	u := insertUser(t, ctx, pool, "multi@example.com")

	insertMembership(t, ctx, pool, orgA, u, "owner")

	_, err := pool.Exec(ctx, `
		INSERT INTO memberships (id, organization_id, user_id, role)
		VALUES ($1, $2, $3, 'employee')`,
		uuid.Must(uuid.NewV7()), orgB, u,
	)
	if err != nil {
		t.Fatalf("ADR-007 requires a user to hold memberships in multiple organizations, but the second insert failed: %v", err)
	}
}

func insertOrganization(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, $2)`, id, name)
	if err != nil {
		t.Fatalf("failed to insert organization: %v", err)
	}
	return id
}

func insertUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Test User')`,
		id, email,
	)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	return id
}

func insertMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orgID, userID uuid.UUID, role string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, $4)`,
		id, orgID, userID, role,
	)
	if err != nil {
		t.Fatalf("failed to insert membership: %v", err)
	}
	return id
}
