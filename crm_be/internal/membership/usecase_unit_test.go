package membership_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/membership/... -run TestUnit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type fakeMember struct {
	membership.Membership
	email    string
	fullName string
}

type fakeRepo struct {
	byID map[uuid.UUID]*fakeMember
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[uuid.UUID]*fakeMember{}}
}

func (f *fakeRepo) Create(_ context.Context, t tenant.Context, id, userID uuid.UUID, role tenant.Role) (*membership.Membership, error) {
	m := &fakeMember{Membership: membership.Membership{ID: id, OrganizationID: t.OrganizationID, UserID: userID, Role: role}}
	f.byID[id] = m
	return &m.Membership, nil
}

func (f *fakeRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*membership.Membership, error) {
	m, ok := f.byID[id]
	if !ok || m.OrganizationID != t.OrganizationID || m.DeletedAt != nil {
		return nil, httpx.ErrNotFound
	}
	return &m.Membership, nil
}

func (f *fakeRepo) FindActiveByUserID(_ context.Context, userID uuid.UUID) ([]*membership.Membership, error) {
	var out []*membership.Membership
	for _, m := range f.byID {
		if m.UserID == userID && m.DeletedAt == nil {
			out = append(out, &m.Membership)
		}
	}
	return out, nil
}

func (f *fakeRepo) FindAllByOrgWithUser(_ context.Context, t tenant.Context) ([]*membership.MemberWithUser, error) {
	var out []*membership.MemberWithUser
	for _, m := range f.byID {
		if m.OrganizationID == t.OrganizationID && m.DeletedAt == nil {
			out = append(out, &membership.MemberWithUser{Membership: m.Membership, Email: m.email, FullName: m.fullName})
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateRole(_ context.Context, t tenant.Context, id uuid.UUID, role tenant.Role) error {
	m, ok := f.byID[id]
	if !ok || m.OrganizationID != t.OrganizationID {
		return httpx.ErrNotFound
	}
	m.Role = role
	return nil
}

func (f *fakeRepo) Deactivate(_ context.Context, t tenant.Context, id uuid.UUID) error {
	m, ok := f.byID[id]
	if !ok || m.OrganizationID != t.OrganizationID {
		return httpx.ErrNotFound
	}
	now := m.CreatedAt // any non-nil time
	m.DeletedAt = &now
	return nil
}

func (f *fakeRepo) CountActiveOwners(_ context.Context, t tenant.Context) (int, error) {
	n := 0
	for _, m := range f.byID {
		if m.OrganizationID == t.OrganizationID && m.Role == tenant.RoleOwner && m.DeletedAt == nil {
			n++
		}
	}
	return n, nil
}

type fakeAuditRepo struct{ actions []string }

func (f *fakeAuditRepo) Record(_ context.Context, _ tenant.Context, _ *uuid.UUID, action string) error {
	f.actions = append(f.actions, action)
	return nil
}

type fakeRefreshTokenRepo struct{ revokedMembershipIDs []uuid.UUID }

func (f *fakeRefreshTokenRepo) RevokeAllByMembershipID(_ context.Context, membershipID uuid.UUID) error {
	f.revokedMembershipIDs = append(f.revokedMembershipIDs, membershipID)
	return nil
}

type fakeStore struct{ repos membership.Repos }

func newFakeStore() *fakeStore {
	return &fakeStore{repos: membership.Repos{
		Member:       newFakeRepo(),
		Audit:        &fakeAuditRepo{},
		RefreshToken: &fakeRefreshTokenRepo{},
	}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(membership.Repos) error) error {
	return fn(s.repos)
}
func (s *fakeStore) Repos() membership.Repos { return s.repos }

func seedMember(store *fakeStore, orgID, userID uuid.UUID, role tenant.Role) uuid.UUID {
	id := uuid.Must(uuid.NewV7())
	_, _ = store.repos.Member.Create(context.Background(), tenant.Context{OrganizationID: orgID}, id, userID, role)
	return id
}

func actorContext(orgID, membershipID uuid.UUID, role tenant.Role) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: role}
}

func TestUnit_List_EmployeeForbidden(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())

	_, err := u.List(context.Background(), actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee))

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden, got: %v", err)
	}
}

func TestUnit_List_ManagerAllowed(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	members, err := u.List(context.Background(), actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleManager))
	if err != nil {
		t.Fatalf("expected manager to be allowed, got: %v", err)
	}
	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

// TestUnit_UpdateRole_SelfChange_Forbidden is rule #3 — universal, no
// exception even for Owner.
func TestUnit_UpdateRole_SelfChange_Forbidden(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownMembershipID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	err := u.UpdateRole(context.Background(), actorContext(orgID, ownMembershipID, tenant.RoleOwner),
		membership.UpdateRoleInput{MembershipID: ownMembershipID, NewRole: tenant.RoleAdmin})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for self role change, got: %v", err)
	}
}

// TestUnit_UpdateRole_AdminCannotTouchOwner is rule #4.
func TestUnit_UpdateRole_AdminCannotTouchOwner(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	adminID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleAdmin)
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	err := u.UpdateRole(context.Background(), actorContext(orgID, adminID, tenant.RoleAdmin),
		membership.UpdateRoleInput{MembershipID: ownerID, NewRole: tenant.RoleAdmin})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for admin touching owner, got: %v", err)
	}
}

// TestUnit_UpdateRole_AdminCannotPromoteToOwner is rule #4's other half.
func TestUnit_UpdateRole_AdminCannotPromoteToOwner(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	adminID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleAdmin)
	employeeID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	err := u.UpdateRole(context.Background(), actorContext(orgID, adminID, tenant.RoleAdmin),
		membership.UpdateRoleInput{MembershipID: employeeID, NewRole: tenant.RoleOwner})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for admin promoting to owner, got: %v", err)
	}
}

// TestUnit_UpdateRole_OwnerCanPromoteToCoOwner proves rule #4 restricts
// only Admin — an Owner creating a co-owner is allowed (no uniqueness
// constraint on the owner count).
func TestUnit_UpdateRole_OwnerCanPromoteToCoOwner(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	adminID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleAdmin)

	err := u.UpdateRole(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner),
		membership.UpdateRoleInput{MembershipID: adminID, NewRole: tenant.RoleOwner})
	if err != nil {
		t.Fatalf("expected owner promoting to co-owner to succeed, got: %v", err)
	}
}

func TestUnit_UpdateRole_CrossOrgTarget_ReturnsNotFound(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgA, orgB := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	ownerA := seedMember(store, orgA, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	targetB := seedMember(store, orgB, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	err := u.UpdateRole(context.Background(), actorContext(orgA, ownerA, tenant.RoleOwner),
		membership.UpdateRoleInput{MembershipID: targetB, NewRole: tenant.RoleAdmin})

	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a cross-org target, got: %v", err)
	}
}

// TestUnit_Deactivate_LastOwner_Returns409 is rule #2.
func TestUnit_Deactivate_LastOwner_Returns409(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), ownerID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "last_owner_cannot_be_removed" {
		t.Fatalf("expected last_owner_cannot_be_removed, got: %v", err)
	}
}

// TestUnit_Deactivate_NotLastOwner_Succeeds proves the restriction is
// specifically about being the LAST owner, not owner-hood itself.
func TestUnit_Deactivate_NotLastOwner_Succeeds(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	owner1 := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner) // second owner

	err := u.Deactivate(context.Background(), actorContext(orgID, owner1, tenant.RoleOwner), owner1)
	if err != nil {
		t.Fatalf("expected deactivation to succeed with a second owner present, got: %v", err)
	}
}

// TestUnit_Deactivate_RevokesRefreshTokens is the direct acceptance
// criterion: deactivation must revoke that membership's sessions in the
// same transaction.
func TestUnit_Deactivate_RevokesRefreshTokens(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	targetID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	if err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	revoker := store.repos.RefreshToken.(*fakeRefreshTokenRepo)
	if len(revoker.revokedMembershipIDs) != 1 || revoker.revokedMembershipIDs[0] != targetID {
		t.Errorf("expected RevokeAllByMembershipID called with %s, got %v", targetID, revoker.revokedMembershipIDs)
	}

	audit := store.repos.Audit.(*fakeAuditRepo)
	if len(audit.actions) != 1 || audit.actions[0] != "membership.deactivated" {
		t.Errorf("expected membership.deactivated audit entry, got %v", audit.actions)
	}
}

func TestUnit_Deactivate_AdminCannotTouchOwner(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	adminID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleAdmin)
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner) // second owner, irrelevant here

	err := u.Deactivate(context.Background(), actorContext(orgID, adminID, tenant.RoleAdmin), ownerID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for admin deactivating owner, got: %v", err)
	}
}
