package device_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/device"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func newTestToken(membershipID uuid.UUID, token string) *device.Token {
	return &device.Token{
		ID:           uuid.Must(uuid.NewV7()),
		MembershipID: membershipID,
		Token:        token,
		Platform:     "android",
	}
}

func TestRepository_Upsert_CreatesNewRow(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := device.New(pool)

	org := seedOrganization(t, ctx, pool)
	membershipID := seedMembership(t, ctx, pool, org, "budi@example.com", tenant.RoleEmployee)
	tc := tenant.Context{OrganizationID: org, MembershipID: &membershipID, Role: tenant.RoleEmployee}

	tok := newTestToken(membershipID, "fcm-token-abc")
	if err := repo.Upsert(ctx, tc, tok); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if tok.CreatedAt.IsZero() || tok.LastSeenAt.IsZero() {
		t.Error("expected CreatedAt/LastSeenAt to be populated after upsert")
	}

	found, err := repo.FindByToken(ctx, tc, "fcm-token-abc")
	if err != nil {
		t.Fatalf("find by token: %v", err)
	}
	if found.MembershipID != membershipID || found.Platform != "android" {
		t.Errorf("unexpected round-tripped token: %+v", found)
	}
}

// TestRepository_Upsert_SameToken_MovesRowToNewOwner is migration
// 0006's own comment, proven: a device that changes hands (employee
// resigns, a new employee logs in on the same physical phone) gets its
// existing row REPOINTED to the new organization/membership, not
// duplicated — because uq_device_tokens_token is unique globally.
func TestRepository_Upsert_SameToken_MovesRowToNewOwner(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := device.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	memberA := seedMembership(t, ctx, pool, orgA, "old-employee@example.com", tenant.RoleEmployee)
	tcA := tenant.Context{OrganizationID: orgA, MembershipID: &memberA, Role: tenant.RoleEmployee}

	first := newTestToken(memberA, "shared-device-token")
	if err := repo.Upsert(ctx, tcA, first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	firstID := first.ID

	orgB := seedOrganization(t, ctx, pool)
	memberB := seedMembership(t, ctx, pool, orgB, "new-employee@example.com", tenant.RoleEmployee)
	tcB := tenant.Context{OrganizationID: orgB, MembershipID: &memberB, Role: tenant.RoleEmployee}

	second := newTestToken(memberB, "shared-device-token") // same token value
	if err := repo.Upsert(ctx, tcB, second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	// Old organization must no longer see this token at all — Rule #6,
	// and the whole point of the move.
	if _, err := repo.FindByToken(ctx, tcA, "shared-device-token"); err != httpx.ErrNotFound {
		t.Errorf("expected old organization to no longer find the moved token, got err=%v", err)
	}

	found, err := repo.FindByToken(ctx, tcB, "shared-device-token")
	if err != nil {
		t.Fatalf("find by token (new owner): %v", err)
	}
	if found.MembershipID != memberB {
		t.Errorf("expected token to belong to new membership %s, got %s", memberB, found.MembershipID)
	}
	if found.ID != firstID {
		t.Errorf("expected the SAME row (id unchanged) to be repointed, got a different id — this means a duplicate row exists")
	}

	rows, err := pool.Query(ctx, `SELECT count(*) FROM device_tokens WHERE token = $1`, "shared-device-token")
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	defer rows.Close()
	var count int
	if rows.Next() {
		_ = rows.Scan(&count)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row for this token globally, got %d — upsert produced a duplicate", count)
	}
}

func TestRepository_FindByToken_CrossOrg_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := device.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	memberA := seedMembership(t, ctx, pool, orgA, "a@example.com", tenant.RoleEmployee)
	tcA := tenant.Context{OrganizationID: orgA, MembershipID: &memberA, Role: tenant.RoleEmployee}
	if err := repo.Upsert(ctx, tcA, newTestToken(memberA, "cross-org-token")); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	orgB := seedOrganization(t, ctx, pool)
	tcB := tenant.Context{OrganizationID: orgB, Role: tenant.RoleOwner}

	if _, err := repo.FindByToken(ctx, tcB, "cross-org-token"); err != httpx.ErrNotFound {
		t.Errorf("expected httpx.ErrNotFound for a token belonging to another organization, got %v", err)
	}
}

func TestRepository_FindByMembership_ReturnsOnlyThatMembersTokens(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := device.New(pool)

	org := seedOrganization(t, ctx, pool)
	memberA := seedMembership(t, ctx, pool, org, "member-a@example.com", tenant.RoleEmployee)
	memberB := seedMembership(t, ctx, pool, org, "member-b@example.com", tenant.RoleEmployee)
	tc := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}

	if err := repo.Upsert(ctx, tc, newTestToken(memberA, "token-a-1")); err != nil {
		t.Fatalf("upsert a1: %v", err)
	}
	if err := repo.Upsert(ctx, tc, newTestToken(memberA, "token-a-2")); err != nil {
		t.Fatalf("upsert a2: %v", err)
	}
	if err := repo.Upsert(ctx, tc, newTestToken(memberB, "token-b-1")); err != nil {
		t.Fatalf("upsert b1: %v", err)
	}

	found, err := repo.FindByMembership(ctx, tc, memberA)
	if err != nil {
		t.Fatalf("find by membership: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2 tokens for memberA, got %d", len(found))
	}
	for _, tok := range found {
		if tok.MembershipID != memberA {
			t.Errorf("expected every result to belong to memberA, got %s", tok.MembershipID)
		}
	}
}

func TestRepository_DeleteByToken_RemovesRow(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := device.New(pool)

	org := seedOrganization(t, ctx, pool)
	memberID := seedMembership(t, ctx, pool, org, "d@example.com", tenant.RoleEmployee)
	tc := tenant.Context{OrganizationID: org, MembershipID: &memberID, Role: tenant.RoleEmployee}

	if err := repo.Upsert(ctx, tc, newTestToken(memberID, "to-delete")); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.DeleteByToken(ctx, tc, "to-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.FindByToken(ctx, tc, "to-delete"); err != httpx.ErrNotFound {
		t.Errorf("expected token to be gone after delete, got err=%v", err)
	}
}

// TestRepository_DeleteByToken_CrossOrg_DoesNotDelete is the repository-
// level half of the tenant isolation this issue's harness case also
// proves at the HTTP layer (cmd/api/tenant_isolation_test.go) — a
// delete scoped to the WRONG organization must affect zero rows, not
// the real one belonging to someone else.
func TestRepository_DeleteByToken_CrossOrg_DoesNotDelete(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := device.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	memberA := seedMembership(t, ctx, pool, orgA, "victim@example.com", tenant.RoleEmployee)
	tcA := tenant.Context{OrganizationID: orgA, MembershipID: &memberA, Role: tenant.RoleEmployee}
	if err := repo.Upsert(ctx, tcA, newTestToken(memberA, "victim-token")); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	orgB := seedOrganization(t, ctx, pool)
	tcB := tenant.Context{OrganizationID: orgB, Role: tenant.RoleOwner}
	if err := repo.DeleteByToken(ctx, tcB, "victim-token"); err != nil {
		t.Fatalf("delete (should be a no-op, not an error): %v", err)
	}

	if _, err := repo.FindByToken(ctx, tcA, "victim-token"); err != nil {
		t.Errorf("expected victim's token to still exist after a cross-org delete attempt, got err=%v", err)
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
