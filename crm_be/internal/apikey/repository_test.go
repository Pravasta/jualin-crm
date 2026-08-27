package apikey_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/apikey"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func newTestKey(keyID string) *apikey.APIKey {
	return &apikey.APIKey{
		ID:         uuid.Must(uuid.NewV7()),
		KeyID:      keyID,
		SecretHash: "test-secret-hash",
		KeyPrefix:  "jln_live_test",
		Name:       "Test Key",
		Scopes:     []string{apikey.ScopeLeadsWrite},
	}
}

func TestRepository_Create_FindByID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := apikey.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}
	k := newTestKey("round-trip-key-id")

	if err := repo.Create(ctx, tenantCtx, k); err != nil {
		t.Fatalf("create: %v", err)
	}
	if k.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be populated after create")
	}

	found, err := repo.FindByID(ctx, tenantCtx, k.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.KeyID != k.KeyID || found.Name != k.Name {
		t.Errorf("expected round-tripped key to match, got %+v", found)
	}
	if len(found.Scopes) != 1 || found.Scopes[0] != apikey.ScopeLeadsWrite {
		t.Errorf("expected scopes [%q], got %v", apikey.ScopeLeadsWrite, found.Scopes)
	}
}

func TestRepository_FindByID_CrossOrg_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := apikey.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	k := newTestKey("cross-org-key-id")
	if err := repo.Create(ctx, tenant.Context{OrganizationID: orgA, Role: tenant.RoleOwner}, k); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := repo.FindByID(ctx, tenant.Context{OrganizationID: orgB, Role: tenant.RoleOwner}, k.ID)
	if err != httpx.ErrNotFound {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestRepository_FindByOrg_NewestFirst_IncludesRevoked(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := apikey.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}

	first := newTestKey("first-key-id")
	if err := repo.Create(ctx, tenantCtx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	second := newTestKey("second-key-id")
	if err := repo.Create(ctx, tenantCtx, second); err != nil {
		t.Fatalf("create second: %v", err)
	}
	if err := repo.Revoke(ctx, tenantCtx, first.ID); err != nil {
		t.Fatalf("revoke first: %v", err)
	}

	keys, err := repo.FindByOrg(ctx, tenantCtx)
	if err != nil {
		t.Fatalf("find by org: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys (revoked ones stay visible, TD §1.1), got %d", len(keys))
	}
	if keys[0].ID != second.ID {
		t.Errorf("expected the most recently created key first, got %+v", keys[0])
	}
	if keys[1].RevokedAt == nil {
		t.Error("expected the first key's RevokedAt to be set")
	}
}

func TestRepository_Revoke_Twice_StaysNilError(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := apikey.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}
	k := newTestKey("revoke-twice-key-id")
	if err := repo.Create(ctx, tenantCtx, k); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.Revoke(ctx, tenantCtx, k.ID); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	firstRevokedAt := mustFindRevokedAt(t, ctx, pool, k.ID)

	if err := repo.Revoke(ctx, tenantCtx, k.ID); err != nil {
		t.Fatalf("second revoke: expected nil error (idempotent), got: %v", err)
	}
	secondRevokedAt := mustFindRevokedAt(t, ctx, pool, k.ID)
	if !firstRevokedAt.Equal(secondRevokedAt) {
		t.Errorf("expected revoked_at to stay fixed at the first revoke (WHERE revoked_at IS NULL made the second UPDATE a no-op), got %v then %v", firstRevokedAt, secondRevokedAt)
	}
}

// TestRepository_Scopes_RejectsUnknownValue proves ck_api_keys_scopes
// blocks a scope outside {leads:write} at the DATABASE level — the
// usecase already rejects this (usecase_unit_test.go), but the migration
// itself is what actually can't be bypassed by a future caller that
// skips the usecase.
func TestRepository_Scopes_RejectsUnknownValue(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := apikey.New(pool)

	org := seedOrganization(t, ctx, pool)
	k := newTestKey("bad-scope-key-id")
	k.Scopes = []string{"leads:read"}

	if err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, k); err == nil {
		t.Fatal("expected ck_api_keys_scopes to reject an unknown scope, got nil error")
	}
}

// TestRepository_CreatedBy_RejectsCrossOrgMembership proves
// fk_api_keys_created_by is a genuine COMPOSITE foreign key — a
// membership from a different organization is rejected by the database,
// not merely unchecked by application code (Rule #3).
func TestRepository_CreatedBy_RejectsCrossOrgMembership(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := apikey.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	memberOfB := seedMembership(t, ctx, pool, orgB, "member-b@example.com", tenant.RoleOwner)

	k := newTestKey("cross-org-created-by-key-id")
	k.CreatedByMembershipID = &memberOfB

	if err := repo.Create(ctx, tenant.Context{OrganizationID: orgA, Role: tenant.RoleOwner}, k); err == nil {
		t.Fatal("expected the composite FK to reject a membership from a different organization, got nil error")
	}
}

// TestRepository_FindByKeyID_IsIndexHit proves the lookup ADR-004's
// verification steps describe is a genuine index hit, not a table scan
// — acceptance criterion #3 of issue #46. enable_seqscan is forced off
// on the acquired connection so the planner's normal "table is tiny,
// just scan it" choice for a near-empty test table doesn't mask whether
// an index-based plan is even AVAILABLE; uq_api_keys_key_id's implicit
// unique index is what's being proven usable here.
func TestRepository_FindByKeyID_IsIndexHit(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := apikey.New(pool)

	org := seedOrganization(t, ctx, pool)
	k := newTestKey("explain-target-key-id")
	if err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, k); err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := repo.FindByKeyID(ctx, k.KeyID)
	if err != nil {
		t.Fatalf("find by key id: %v", err)
	}
	if found.ID != k.ID {
		t.Fatalf("expected FindByKeyID to return the seeded key, got %+v", found)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SET enable_seqscan = off"); err != nil {
		t.Fatalf("SET enable_seqscan = off: %v", err)
	}

	rows, err := conn.Query(ctx, `EXPLAIN SELECT id FROM api_keys WHERE key_id = $1`, k.KeyID)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("explain rows: %v", err)
	}

	plan := strings.Join(planLines, "\n")
	if !strings.Contains(plan, "Index Scan") && !strings.Contains(plan, "Index Only Scan") {
		t.Fatalf("expected an index-based plan for WHERE key_id = $1, got:\n%s", plan)
	}
}

func mustFindRevokedAt(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) time.Time {
	t.Helper()
	var revokedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT revoked_at FROM api_keys WHERE id = $1`, id).Scan(&revokedAt); err != nil {
		t.Fatalf("query revoked_at: %v", err)
	}
	if revokedAt == nil {
		t.Fatal("expected revoked_at to be set")
	}
	return *revokedAt
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
