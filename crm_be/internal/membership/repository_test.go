package membership_test

import (
	"context"
	"errors"
	"fmt"
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

// --- FindActiveOwnerIDs (Phase 8.5 #123) ---

func TestRepository_FindActiveOwnerIDs_ReturnsEveryActiveOwner(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	org := seedOrganization(t, ctx, pool)
	t1 := tenant.Context{OrganizationID: org}

	owner1 := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, t1, owner1, seedUser(t, ctx, pool, "owner1@example.com"), tenant.RoleOwner); err != nil {
		t.Fatalf("seed owner1: %v", err)
	}
	// Co-owner — Aturan #4's own note allows this (not restricted the
	// way Admin promoting to Owner is), so BOTH must come back.
	owner2 := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, t1, owner2, seedUser(t, ctx, pool, "owner2@example.com"), tenant.RoleOwner); err != nil {
		t.Fatalf("seed owner2: %v", err)
	}
	admin := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, t1, admin, seedUser(t, ctx, pool, "admin@example.com"), tenant.RoleAdmin); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	ids, err := repo.FindActiveOwnerIDs(ctx, t1)
	if err != nil {
		t.Fatalf("find active owner ids: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 owners, got %d: %v", len(ids), ids)
	}
	got := map[uuid.UUID]bool{ids[0]: true, ids[1]: true}
	if !got[owner1] || !got[owner2] {
		t.Errorf("expected both owners %s and %s, got %v", owner1, owner2, ids)
	}
	if got[admin] {
		t.Error("expected the admin to be excluded")
	}
}

func TestRepository_FindActiveOwnerIDs_ExcludesDeactivated(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	org := seedOrganization(t, ctx, pool)
	t1 := tenant.Context{OrganizationID: org}

	activeOwner := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, t1, activeOwner, seedUser(t, ctx, pool, "active-owner@example.com"), tenant.RoleOwner); err != nil {
		t.Fatalf("seed active owner: %v", err)
	}
	deactivatedOwner := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, t1, deactivatedOwner, seedUser(t, ctx, pool, "gone-owner@example.com"), tenant.RoleOwner); err != nil {
		t.Fatalf("seed deactivated owner: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE memberships SET deleted_at = now() WHERE id = $1`, deactivatedOwner); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	ids, err := repo.FindActiveOwnerIDs(ctx, t1)
	if err != nil {
		t.Fatalf("find active owner ids: %v", err)
	}
	if len(ids) != 1 || ids[0] != activeOwner {
		t.Errorf("expected only the active owner %s, got %v", activeOwner, ids)
	}
}

func TestRepository_FindActiveOwnerIDs_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	if _, err := repo.Create(ctx, tenant.Context{OrganizationID: orgB}, uuid.Must(uuid.NewV7()), seedUser(t, ctx, pool, "owner-b@example.com"), tenant.RoleOwner); err != nil {
		t.Fatalf("seed org B owner: %v", err)
	}

	ids, err := repo.FindActiveOwnerIDs(ctx, tenant.Context{OrganizationID: orgA})
	if err != nil {
		t.Fatalf("find active owner ids: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected org A to see 0 of org B's owners, got %v", ids)
	}
}

// --- CountActive (#122, closing the coverage gap #124 depends on) ---

func TestRepository_CountActive_CountsAcrossRoles(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	org := seedOrganization(t, ctx, pool)
	t1 := tenant.Context{OrganizationID: org}

	roles := []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin, tenant.RoleManager, tenant.RoleEmployee}
	for i, role := range roles {
		if _, err := repo.Create(ctx, t1, uuid.Must(uuid.NewV7()), seedUser(t, ctx, pool, fmt.Sprintf("member-%d@example.com", i)), role); err != nil {
			t.Fatalf("seed %s: %v", role, err)
		}
	}

	n, err := repo.CountActive(ctx, t1)
	if err != nil {
		t.Fatalf("count active: %v", err)
	}
	if n != len(roles) {
		t.Errorf("expected %d active memberships across every role, got %d", len(roles), n)
	}
}

func TestRepository_CountActive_ExcludesDeactivated(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	org := seedOrganization(t, ctx, pool)
	t1 := tenant.Context{OrganizationID: org}

	active := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, t1, active, seedUser(t, ctx, pool, "active@example.com"), tenant.RoleEmployee); err != nil {
		t.Fatalf("seed active: %v", err)
	}
	deactivated := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, t1, deactivated, seedUser(t, ctx, pool, "gone@example.com"), tenant.RoleEmployee); err != nil {
		t.Fatalf("seed deactivated: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE memberships SET deleted_at = now() WHERE id = $1`, deactivated); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	n, err := repo.CountActive(ctx, t1)
	if err != nil {
		t.Fatalf("count active: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 active membership, got %d", n)
	}
}

func TestRepository_CountActive_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := membership.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	if _, err := repo.Create(ctx, tenant.Context{OrganizationID: orgB}, uuid.Must(uuid.NewV7()), seedUser(t, ctx, pool, "member-b@example.com"), tenant.RoleEmployee); err != nil {
		t.Fatalf("seed org B member: %v", err)
	}

	n, err := repo.CountActive(ctx, tenant.Context{OrganizationID: orgA})
	if err != nil {
		t.Fatalf("count active: %v", err)
	}
	if n != 0 {
		t.Errorf("expected org A to count 0 of org B's members, got %d", n)
	}
}
