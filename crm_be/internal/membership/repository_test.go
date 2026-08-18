package membership_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func TestRepository_CreateAndFindByID(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	org := seedOrganization(t, ctx, pool)
	u := seedUser(t, ctx, pool, "owner@example.com")
	t1 := tenant.Context{OrganizationID: org}

	id := uuid.Must(uuid.NewV7())
	created, err := repo.Create(ctx, t1, id, u, tenant.RoleOwner)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.Role != tenant.RoleOwner {
		t.Errorf("expected role owner, got %s", created.Role)
	}

	found, err := repo.FindByID(ctx, t1, id)
	if err != nil {
		t.Fatalf("find by id failed: %v", err)
	}
	if found.ID != id || found.OrganizationID != org || found.UserID != u {
		t.Errorf("found membership doesn't match created one: %+v", found)
	}
}

// TestRepository_FindByID_CrossTenant_ReturnsNotFound is the Go-level
// counterpart to db_test's raw-SQL composite FK test — it proves the
// tenant-scoping pattern itself (not just the database constraint)
// behaves per Rule #6: a membership belonging to another organization
// is indistinguishable from one that doesn't exist. 404, never 403.
func TestRepository_FindByID_CrossTenant_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	userA := seedUser(t, ctx, pool, "a@example.com")

	id := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, tenant.Context{OrganizationID: orgA}, id, userA, tenant.RoleEmployee); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Same membership id, but requested under orgB's tenant context.
	_, err := repo.FindByID(ctx, tenant.Context{OrganizationID: orgB}, id)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a membership in a different organization, got: %v", err)
	}
}

// TestRepository_FindActiveByUserID_SpansOrganizations exercises the one
// query that is intentionally NOT organization-scoped — the login flow
// ADR-007's multi-membership model depends on.
func TestRepository_FindActiveByUserID_SpansOrganizations(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	u := seedUser(t, ctx, pool, "multi@example.com")

	if _, err := repo.Create(ctx, tenant.Context{OrganizationID: orgA}, uuid.Must(uuid.NewV7()), u, tenant.RoleOwner); err != nil {
		t.Fatalf("create in org A failed: %v", err)
	}
	if _, err := repo.Create(ctx, tenant.Context{OrganizationID: orgB}, uuid.Must(uuid.NewV7()), u, tenant.RoleEmployee); err != nil {
		t.Fatalf("create in org B failed: %v", err)
	}

	found, err := repo.FindActiveByUserID(ctx, u)
	if err != nil {
		t.Fatalf("find active by user id failed: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2 memberships across organizations, got %d", len(found))
	}
}

func seedOrganization(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Test Org')`, id); err != nil {
		t.Fatalf("failed to seed organization: %v", err)
	}
	return id
}

func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Test User')`,
		id, email,
	)
	if err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return id
}
