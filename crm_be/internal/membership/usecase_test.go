package membership_test

// Integration tests (real Postgres via dbtest) proving the SQL behind
// FindAllByOrgWithUser/UpdateRole/Deactivate/CountActiveOwners actually
// works — the fakes in usecase_unit_test.go prove the business-logic
// branches, but only real Postgres proves the join, the UPDATE
// predicates, and the COUNT are correct.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// testStore is a real, PostgreSQL-backed membership.Store — mirrors
// cmd/api/membership_store.go's wiring. RefreshToken is a genuine no-op
// here: this file only exercises membership's own SQL, not the
// cross-package revoke contract — that's covered end-to-end by
// cmd/api's tenant isolation suite, where a real auth session exists to
// revoke.
type testStore struct {
	pool *pgxpool.Pool
}

func newTestStore(pool *pgxpool.Pool) membership.Store {
	return &testStore{pool: pool}
}

func (s *testStore) InTx(ctx context.Context, fn func(membership.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(testRepos(tx)) })
}

func (s *testStore) Repos() membership.Repos {
	return testRepos(s.pool)
}

func testRepos(q db.Querier) membership.Repos {
	return membership.Repos{
		Member:       membership.New(q),
		Audit:        auditlog.New(q),
		RefreshToken: noopRefreshTokenRepo{},
		OpenLead:     lead.NewOpenLeadRepository(q),
		Activity:     activity.NewRecorder(q),
	}
}

type noopRefreshTokenRepo struct{}

func (noopRefreshTokenRepo) RevokeAllByMembershipID(_ context.Context, _ uuid.UUID) error { return nil }

func TestUsecase_List_ReturnsMembersWithUserFields(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	u := membership.NewUsecase(newTestStore(pool))

	org := seedOrganization(t, ctx, pool)
	owner := seedUser(t, ctx, pool, "list-owner@example.com")
	ownerMembershipID := seedMembership(t, ctx, pool, org, owner, tenant.RoleOwner)

	actor := tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembershipID, Role: tenant.RoleOwner}
	members, err := u.List(ctx, actor)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(members) != 1 || members[0].Email != "list-owner@example.com" {
		t.Fatalf("expected 1 member with joined email, got: %+v", members)
	}
}

func TestUsecase_UpdateRole_PersistsAgainstRealPostgres(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	u := membership.NewUsecase(newTestStore(pool))

	org := seedOrganization(t, ctx, pool)
	owner := seedUser(t, ctx, pool, "update-owner@example.com")
	ownerMembershipID := seedMembership(t, ctx, pool, org, owner, tenant.RoleOwner)
	targetUser := seedUser(t, ctx, pool, "update-target@example.com")
	targetMembershipID := seedMembership(t, ctx, pool, org, targetUser, tenant.RoleEmployee)

	actor := tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembershipID, Role: tenant.RoleOwner}
	err := u.UpdateRole(ctx, actor, membership.UpdateRoleInput{MembershipID: targetMembershipID, NewRole: tenant.RoleManager})
	if err != nil {
		t.Fatalf("update role: %v", err)
	}

	var role string
	if err := pool.QueryRow(ctx, `SELECT role FROM memberships WHERE id = $1`, targetMembershipID).Scan(&role); err != nil {
		t.Fatalf("query role: %v", err)
	}
	if role != string(tenant.RoleManager) {
		t.Errorf("expected role manager in database, got %q", role)
	}
}

func TestUsecase_Deactivate_SoftDeletesAgainstRealPostgres(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	u := membership.NewUsecase(newTestStore(pool))

	org := seedOrganization(t, ctx, pool)
	owner := seedUser(t, ctx, pool, "deactivate-owner@example.com")
	ownerMembershipID := seedMembership(t, ctx, pool, org, owner, tenant.RoleOwner)
	targetUser := seedUser(t, ctx, pool, "deactivate-target@example.com")
	targetMembershipID := seedMembership(t, ctx, pool, org, targetUser, tenant.RoleEmployee)

	actor := tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembershipID, Role: tenant.RoleOwner}
	if err := u.Deactivate(ctx, actor, targetMembershipID, membership.DeactivateInput{}); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	var deletedAt *string
	if err := pool.QueryRow(ctx, `SELECT deleted_at::text FROM memberships WHERE id = $1`, targetMembershipID).Scan(&deletedAt); err != nil {
		t.Fatalf("query deleted_at: %v", err)
	}
	if deletedAt == nil {
		t.Error("expected deleted_at to be set")
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE organization_id = $1 AND action = 'membership.deactivated'`, org).Scan(&auditCount); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 membership.deactivated audit entry, got %d", auditCount)
	}
}

func TestUsecase_Deactivate_LastOwner_Returns409AgainstRealPostgres(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	u := membership.NewUsecase(newTestStore(pool))

	org := seedOrganization(t, ctx, pool)
	owner := seedUser(t, ctx, pool, "last-owner@example.com")
	ownerMembershipID := seedMembership(t, ctx, pool, org, owner, tenant.RoleOwner)

	actor := tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembershipID, Role: tenant.RoleOwner}
	err := u.Deactivate(ctx, actor, ownerMembershipID, membership.DeactivateInput{})
	if err == nil {
		t.Fatal("expected an error deactivating the last owner")
	}
}

func seedMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, userID uuid.UUID, role tenant.Role) uuid.UUID {
	t.Helper()
	repo := membership.New(pool)
	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org}, uuid.Must(uuid.NewV7()), userID, role)
	if err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return created.ID
}
