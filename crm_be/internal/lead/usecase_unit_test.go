package lead_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/lead/... -run TestUnit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeLeadRepo struct {
	byID       map[uuid.UUID]*lead.Lead
	nextNumber int
}

func newFakeLeadRepo() *fakeLeadRepo {
	return &fakeLeadRepo{byID: map[uuid.UUID]*lead.Lead{}, nextNumber: 1}
}

func (f *fakeLeadRepo) Create(_ context.Context, t tenant.Context, in lead.CreateInput) (*lead.Lead, error) {
	if in.IdempotencyKey != nil {
		for _, l := range f.byID {
			if l.IdempotencyKey != nil && *l.IdempotencyKey == *in.IdempotencyKey && l.OrganizationID == t.OrganizationID {
				return nil, lead.ErrIdempotencyKeyExists
			}
		}
	}
	l := &lead.Lead{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: t.OrganizationID, LeadNumber: f.nextNumber,
		Name: in.Name, Email: in.Email, Phone: in.Phone, PhoneE164: in.PhoneE164, Company: in.Company, Notes: in.Notes,
		Status: "new", Source: in.Source, AssignedToMembershipID: in.AssignedToMembershipID,
		IdempotencyKey: in.IdempotencyKey, Version: 1, CreatedByMembershipID: in.CreatedByMembershipID,
	}
	f.nextNumber++
	f.byID[l.ID] = l
	return l, nil
}

func (f *fakeLeadRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*lead.Lead, error) {
	l, ok := f.byID[id]
	if !ok || l.OrganizationID != t.OrganizationID || l.DeletedAt != nil {
		return nil, httpx.ErrNotFound
	}
	if t.Role == tenant.RoleEmployee && (l.AssignedToMembershipID == nil || t.MembershipID == nil || *l.AssignedToMembershipID != *t.MembershipID) {
		return nil, httpx.ErrNotFound
	}
	return l, nil
}

func (f *fakeLeadRepo) FindByIdempotencyKey(_ context.Context, t tenant.Context, key string) (*lead.Lead, error) {
	for _, l := range f.byID {
		if l.OrganizationID == t.OrganizationID && l.IdempotencyKey != nil && *l.IdempotencyKey == key {
			return l, nil
		}
	}
	return nil, httpx.ErrNotFound
}

func (f *fakeLeadRepo) FindAllByOrg(_ context.Context, t tenant.Context, _ lead.ListFilter) ([]*lead.Lead, int, error) {
	var out []*lead.Lead
	for _, l := range f.byID {
		if l.OrganizationID == t.OrganizationID && l.DeletedAt == nil {
			out = append(out, l)
		}
	}
	return out, len(out), nil
}

func (f *fakeLeadRepo) Update(_ context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in lead.UpdateInput) (*lead.Lead, error) {
	l, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if l.Version != expectedVersion {
		return l, lead.ErrVersionConflict
	}
	if in.Name != nil {
		l.Name = *in.Name
	}
	if in.Email != nil {
		l.Email = in.Email
	}
	l.Version++
	return l, nil
}

func (f *fakeLeadRepo) UpdateStatus(_ context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, status string, lostReason *string) (*lead.Lead, error) {
	l, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if l.Version != expectedVersion {
		return l, lead.ErrVersionConflict
	}
	l.Status = status
	l.LostReason = lostReason
	l.Version++
	return l, nil
}

func (f *fakeLeadRepo) Delete(_ context.Context, t tenant.Context, id uuid.UUID) error {
	l, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return err
	}
	now := l.CreatedAt
	l.DeletedAt = &now
	return nil
}

type fakeStore struct{ repos lead.Repos }

func newFakeStore() *fakeStore {
	return &fakeStore{repos: lead.Repos{Lead: newFakeLeadRepo()}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(lead.Repos) error) error { return fn(s.repos) }
func (s *fakeStore) Repos() lead.Repos                                       { return s.repos }

func actorContext(orgID, membershipID uuid.UUID, role tenant.Role) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: role}
}

func ownerActor() (tenant.Context, uuid.UUID) {
	org := uuid.Must(uuid.NewV7())
	return actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner), org
}

// --- tests ---

func TestUnit_Create_EmployeeForbidden(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	org := uuid.Must(uuid.NewV7())

	_, _, err := u.Create(context.Background(), actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee), lead.CreateLeadInput{Name: "Budi"})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden, got: %v", err)
	}
}

func TestUnit_Create_NameRequired(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	_, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got: %v", err)
	}
}

func TestUnit_Create_DefaultsSourceToManual(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	created, isNew, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true for a fresh create")
	}
	if created.Source != "manual" {
		t.Errorf("expected default source 'manual', got %q", created.Source)
	}
}

func TestUnit_Create_InvalidSource_Rejected(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	_, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", Source: "carrier-pigeon"})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for an invalid source, got: %v", err)
	}
}

func TestUnit_Create_NormalizesEmailAndPhone(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	email := "Budi@Example.COM"
	phoneRaw := "0812-3456-7890"
	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", Email: &email, Phone: &phoneRaw})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Email == nil || *created.Email != "budi@example.com" {
		t.Errorf("expected lowercased email, got %v", created.Email)
	}
	if created.PhoneE164 == nil || *created.PhoneE164 != "+6281234567890" {
		t.Errorf("expected normalized phone_e164, got %v", created.PhoneE164)
	}
	if created.Phone == nil || *created.Phone != phoneRaw {
		t.Errorf("expected raw phone preserved, got %v", created.Phone)
	}
}

func TestUnit_Create_UnparseablePhone_StillAccepted(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	badPhone := "1234"
	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", Phone: &badPhone})
	if err != nil {
		t.Fatalf("expected an unparseable phone to still be accepted, got: %v", err)
	}
	if created.PhoneE164 != nil {
		t.Errorf("expected nil phone_e164 for an unparseable number, got %v", *created.PhoneE164)
	}
	if created.Phone == nil || *created.Phone != badPhone {
		t.Error("expected the raw phone value to be preserved even when unparseable")
	}
}

func TestUnit_Create_IdempotentReplay_ReturnsSameLeadNotNew(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	key := "client-generated-key"
	first, isNew1, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", IdempotencyKey: &key})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if !isNew1 {
		t.Fatal("expected the first create to be new")
	}

	second, isNew2, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Different Name Ignored", IdempotencyKey: &key})
	if err != nil {
		t.Fatalf("replay create: %v", err)
	}
	if isNew2 {
		t.Error("expected the replay to report isNew=false")
	}
	if second.ID != first.ID {
		t.Errorf("expected the replay to return the SAME lead, got a different id")
	}
}

func TestUnit_Get_Employee_OtherPersonsLead_ReturnsNotFound(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, org := ownerActor()

	otherEmployee := uuid.Must(uuid.NewV7())
	assignedIn := lead.CreateLeadInput{Name: "Assigned Elsewhere", AssignedToMembershipID: &otherEmployee}
	created, _, err := u.Create(context.Background(), actor, assignedIn)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	me := uuid.Must(uuid.NewV7())
	_, err = u.Get(context.Background(), actorContext(org, me, tenant.RoleEmployee), created.ID)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestUnit_Update_StaleVersion_ReturnsVersionConflict(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "Updated"
	_, err = u.Update(context.Background(), actor, created.ID, lead.UpdateLeadInput{Version: created.Version - 1, Name: &newName})

	var conflict *lead.VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *lead.VersionConflictError, got: %v", err)
	}
	if conflict.Current == nil || conflict.Current.ID != created.ID {
		t.Error("expected the conflict to carry the current lead")
	}
}

func TestUnit_Delete_ManagerForbidden(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, org := ownerActor()

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	managerCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleManager)
	err = u.Delete(context.Background(), managerCtx, created.ID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for manager delete, got: %v", err)
	}
}

// --- status transition tests (TD §5) ---

func TestUnit_UpdateStatus_NewToWon_Rejected(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	_, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "won"})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_status_transition" {
		t.Fatalf("expected invalid_status_transition, got: %v", err)
	}
}

func TestUnit_UpdateStatus_NewToContacted_Accepted(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	updated, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "contacted"})
	if err != nil {
		t.Fatalf("expected new->contacted to succeed, got: %v", err)
	}
	if updated.Status != "contacted" {
		t.Errorf("expected status contacted, got %q", updated.Status)
	}
}

func TestUnit_UpdateStatus_QualifiedToContacted_Accepted(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	step1, _ := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "contacted"})
	step2, _ := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: step1.Version, Status: "qualified"})

	updated, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: step2.Version, Status: "contacted"})
	if err != nil {
		t.Fatalf("expected qualified->contacted (one step back, B7) to succeed, got: %v", err)
	}
	if updated.Status != "contacted" {
		t.Errorf("expected status contacted, got %q", updated.Status)
	}
}

func TestUnit_UpdateStatus_Lost_RequiresReason(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	_, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "lost"})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for lost without a reason, got: %v", err)
	}
}

func TestUnit_UpdateStatus_LostWithReason_Accepted(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	reason := "price"
	updated, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "lost", LostReason: &reason})
	if err != nil {
		t.Fatalf("expected lost with a reason to succeed, got: %v", err)
	}
	if updated.LostReason == nil || *updated.LostReason != "price" {
		t.Errorf("expected lost_reason 'price', got %v", updated.LostReason)
	}
}

func TestUnit_UpdateStatus_LeavingLost_ClearsReason(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	reason := "timing"
	lost, _ := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "lost", LostReason: &reason})

	// Approximation documented in usecase.go: leaving lost allows any
	// main-path status, not specifically the one it left from.
	revived, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: lost.Version, Status: "qualified"})
	if err != nil {
		t.Fatalf("expected leaving lost to succeed, got: %v", err)
	}
	if revived.LostReason != nil {
		t.Errorf("expected lost_reason to be cleared, got %v", *revived.LostReason)
	}
}

func TestUnit_UpdateStatus_UnqualifiedIsFinal(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	unqualified, _ := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "unqualified"})

	_, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: unqualified.Version, Status: "new"})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_status_transition" {
		t.Fatalf("expected unqualified to be final (invalid_status_transition), got: %v", err)
	}
}
