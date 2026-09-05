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

// CountActive is CountActiveOwners without the role predicate — half the
// seat meter (#122). No test in this file reads it yet; enforcement is
// #124's.
func (f *fakeRepo) CountActive(_ context.Context, t tenant.Context) (int, error) {
	n := 0
	for _, m := range f.byID {
		if m.OrganizationID == t.OrganizationID && m.DeletedAt == nil {
			n++
		}
	}
	return n, nil
}

// FindActiveOwnerIDs exists so *fakeRepo still satisfies
// membership.Repository (#123). No test in this file reads it yet —
// notification is exercised where the composition root wires it.
func (f *fakeRepo) FindActiveOwnerIDs(_ context.Context, t tenant.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for _, m := range f.byID {
		if m.OrganizationID == t.OrganizationID && m.Role == tenant.RoleOwner && m.DeletedAt == nil {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
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

// fakeOpenLeadRepo lets tests control how many open leads a membership
// "owns" and assert which of Unassign/Reassign got called with what.
type fakeOpenLeadRepo struct {
	openCount     int
	unassignCalls []uuid.UUID
	reassignCalls [][2]uuid.UUID // [membershipID, reassignTo]
	openLeadIDs   []uuid.UUID
	reassignErr   error
}

func (f *fakeOpenLeadRepo) CountOpen(_ context.Context, _ tenant.Context, _ uuid.UUID) (int, error) {
	return f.openCount, nil
}

func (f *fakeOpenLeadRepo) UnassignOpen(_ context.Context, _ tenant.Context, membershipID uuid.UUID) ([]uuid.UUID, error) {
	f.unassignCalls = append(f.unassignCalls, membershipID)
	return f.openLeadIDs, nil
}

func (f *fakeOpenLeadRepo) ReassignOpen(_ context.Context, _ tenant.Context, membershipID, reassignTo uuid.UUID) ([]uuid.UUID, error) {
	f.reassignCalls = append(f.reassignCalls, [2]uuid.UUID{membershipID, reassignTo})
	if f.reassignErr != nil {
		return nil, f.reassignErr
	}
	return f.openLeadIDs, nil
}

type recordedMembershipActivity struct {
	leadID       uuid.UUID
	activityType string
	metadata     map[string]any
}

type fakeMembershipActivityRecorder struct{ calls []recordedMembershipActivity }

func (f *fakeMembershipActivityRecorder) Record(_ context.Context, _ tenant.Context, leadID uuid.UUID, activityType string, _ *uuid.UUID, metadata map[string]any) error {
	f.calls = append(f.calls, recordedMembershipActivity{leadID, activityType, metadata})
	return nil
}

type fakeStore struct {
	repos    membership.Repos
	openLead *fakeOpenLeadRepo
	activity *fakeMembershipActivityRecorder
}

func newFakeStore() *fakeStore {
	openLead := &fakeOpenLeadRepo{}
	act := &fakeMembershipActivityRecorder{}
	return &fakeStore{
		repos: membership.Repos{
			Member:       newFakeRepo(),
			Audit:        &fakeAuditRepo{},
			RefreshToken: &fakeRefreshTokenRepo{},
			OpenLead:     openLead,
			Activity:     act,
		},
		openLead: openLead,
		activity: act,
	}
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

	err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), ownerID, membership.DeactivateInput{})

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

	err := u.Deactivate(context.Background(), actorContext(orgID, owner1, tenant.RoleOwner), owner1, membership.DeactivateInput{})
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

	if err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID, membership.DeactivateInput{}); err != nil {
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

	err := u.Deactivate(context.Background(), actorContext(orgID, adminID, tenant.RoleAdmin), ownerID, membership.DeactivateInput{})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for admin deactivating owner, got: %v", err)
	}
}

// --- on_open_leads tests (TD §13) ---

// TestUnit_Deactivate_OpenLeads_Reject_BlocksAndDoesNotDeactivate is the
// default path's core guarantee: the whole transaction aborts before
// Member.Deactivate ever runs — provable on the fake by asserting the
// membership is still active afterward.
func TestUnit_Deactivate_OpenLeads_Reject_BlocksAndDoesNotDeactivate(t *testing.T) {
	store := newFakeStore()
	store.openLead.openCount = 2
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	targetID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID, membership.DeactivateInput{})

	var openLeads *membership.OpenLeadsError
	if !errors.As(err, &openLeads) || openLeads.Count != 2 {
		t.Fatalf("expected *OpenLeadsError{Count: 2}, got: %v", err)
	}

	found, findErr := store.repos.Member.FindByID(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID)
	if findErr != nil {
		t.Fatalf("find target: %v", findErr)
	}
	if found.DeletedAt != nil {
		t.Error("expected the membership to remain active after a rejected deactivation")
	}
	if len(store.activity.calls) != 0 {
		t.Errorf("expected no activity recorded on the rejected path, got %d", len(store.activity.calls))
	}
}

// TestUnit_Deactivate_OpenLeads_DefaultIsReject proves an empty
// OnOpenLeads behaves identically to explicit "reject" — the zero value
// of DeactivateInput must be the safe default.
func TestUnit_Deactivate_OpenLeads_DefaultIsReject(t *testing.T) {
	store := newFakeStore()
	store.openLead.openCount = 1
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	targetID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID, membership.DeactivateInput{OnOpenLeads: ""})

	var openLeads *membership.OpenLeadsError
	if !errors.As(err, &openLeads) {
		t.Fatalf("expected empty OnOpenLeads to default to reject, got: %v", err)
	}
}

func TestUnit_Deactivate_OpenLeads_Unassign_RecordsOneActivityPerLead(t *testing.T) {
	store := newFakeStore()
	leadA, leadB := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	store.openLead.openLeadIDs = []uuid.UUID{leadA, leadB}
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	targetID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID,
		membership.DeactivateInput{OnOpenLeads: "unassign"})
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	if len(store.openLead.unassignCalls) != 1 || store.openLead.unassignCalls[0] != targetID {
		t.Fatalf("expected UnassignOpen called once with %s, got %v", targetID, store.openLead.unassignCalls)
	}
	if len(store.activity.calls) != 2 {
		t.Fatalf("expected 2 lead_unassigned activities, got %d", len(store.activity.calls))
	}
	for _, call := range store.activity.calls {
		if call.activityType != "lead_unassigned" || call.metadata["from"] != targetID {
			t.Errorf("expected lead_unassigned with from=%s, got %+v", targetID, call)
		}
	}

	// FindByID filters out deleted rows, so check the underlying fake
	// directly — a deactivated membership is expected to disappear from
	// FindByID by design (Rule #6-adjacent: it's just gone, not an error).
	stored := store.repos.Member.(*fakeRepo).byID[targetID]
	if stored.DeletedAt == nil {
		t.Error("expected the membership to be deactivated on the unassign path")
	}
}

func TestUnit_Deactivate_OpenLeads_Reassign_RecordsLeadAssigned(t *testing.T) {
	store := newFakeStore()
	leadA := uuid.Must(uuid.NewV7())
	store.openLead.openLeadIDs = []uuid.UUID{leadA}
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	targetID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	reassignTo := uuid.Must(uuid.NewV7())

	err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID,
		membership.DeactivateInput{OnOpenLeads: "reassign", ReassignTo: &reassignTo})
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	if len(store.openLead.reassignCalls) != 1 || store.openLead.reassignCalls[0] != [2]uuid.UUID{targetID, reassignTo} {
		t.Fatalf("expected ReassignOpen called once with (%s, %s), got %v", targetID, reassignTo, store.openLead.reassignCalls)
	}
	if len(store.activity.calls) != 1 || store.activity.calls[0].activityType != "lead_assigned" {
		t.Fatalf("expected 1 lead_assigned activity, got %+v", store.activity.calls)
	}
	if store.activity.calls[0].metadata["from"] != targetID || store.activity.calls[0].metadata["to"] != reassignTo {
		t.Errorf("expected metadata {from: %s, to: %s}, got %v", targetID, reassignTo, store.activity.calls[0].metadata)
	}
}

func TestUnit_Deactivate_OpenLeads_Reassign_RequiresReassignTo(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	targetID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID,
		membership.DeactivateInput{OnOpenLeads: "reassign"})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for reassign without reassign_to, got: %v", err)
	}
}

func TestUnit_Deactivate_OpenLeads_InvalidValue_Rejected(t *testing.T) {
	store := newFakeStore()
	u := membership.NewUsecase(store)
	orgID := uuid.Must(uuid.NewV7())
	ownerID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	targetID := seedMember(store, orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	err := u.Deactivate(context.Background(), actorContext(orgID, ownerID, tenant.RoleOwner), targetID,
		membership.DeactivateInput{OnOpenLeads: "delete-them-all"})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for an invalid on_open_leads value, got: %v", err)
	}
}
