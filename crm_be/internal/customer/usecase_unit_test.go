package customer_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/customer/... -run TestUnit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/customer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeCustomerRepo struct {
	byID             map[uuid.UUID]*customer.Customer
	convertedLeadIDs map[uuid.UUID]bool
	leadStatus       map[uuid.UUID]string // simulates lead visibility+status for Convert
}

func newFakeCustomerRepo() *fakeCustomerRepo {
	return &fakeCustomerRepo{
		byID:             map[uuid.UUID]*customer.Customer{},
		convertedLeadIDs: map[uuid.UUID]bool{},
		leadStatus:       map[uuid.UUID]string{},
	}
}

func (f *fakeCustomerRepo) Convert(_ context.Context, t tenant.Context, leadID uuid.UUID, convertedBy *uuid.UUID) (*customer.Customer, error) {
	status, exists := f.leadStatus[leadID]
	if !exists {
		return nil, httpx.ErrNotFound
	}
	if status != "won" {
		return nil, customer.ErrLeadNotWon
	}
	if f.convertedLeadIDs[leadID] {
		return nil, customer.ErrAlreadyConverted
	}
	c := &customer.Customer{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: t.OrganizationID, Name: "Converted",
		ConvertedFromLeadID: leadID, ConvertedByMembershipID: convertedBy,
	}
	f.byID[c.ID] = c
	f.convertedLeadIDs[leadID] = true
	return c, nil
}

func (f *fakeCustomerRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*customer.Customer, error) {
	c, ok := f.byID[id]
	if !ok || c.OrganizationID != t.OrganizationID || c.DeletedAt != nil {
		return nil, httpx.ErrNotFound
	}
	return c, nil
}

func (f *fakeCustomerRepo) FindAllByOrg(_ context.Context, t tenant.Context, _ customer.ListFilter) ([]*customer.Customer, int, error) {
	var out []*customer.Customer
	for _, c := range f.byID {
		if c.OrganizationID == t.OrganizationID && c.DeletedAt == nil {
			out = append(out, c)
		}
	}
	return out, len(out), nil
}

func (f *fakeCustomerRepo) Update(_ context.Context, t tenant.Context, id uuid.UUID, in customer.UpdateInput) (*customer.Customer, error) {
	c, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		c.Name = *in.Name
	}
	if in.Email != nil {
		c.Email = in.Email
	}
	return c, nil
}

func (f *fakeCustomerRepo) Delete(_ context.Context, t tenant.Context, id uuid.UUID) error {
	c, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return err
	}
	now := c.CreatedAt
	c.DeletedAt = &now
	return nil
}

type recordedActivity struct {
	leadID       uuid.UUID
	activityType string
	metadata     map[string]any
}

type fakeActivityRecorder struct{ calls []recordedActivity }

func (f *fakeActivityRecorder) Record(_ context.Context, _ tenant.Context, leadID uuid.UUID, activityType string, _ *uuid.UUID, metadata map[string]any) error {
	f.calls = append(f.calls, recordedActivity{leadID, activityType, metadata})
	return nil
}

type fakeStore struct {
	repo     *fakeCustomerRepo
	activity *fakeActivityRecorder
}

func newFakeStore() *fakeStore {
	return &fakeStore{repo: newFakeCustomerRepo(), activity: &fakeActivityRecorder{}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(customer.Repos) error) error {
	return fn(customer.Repos{Customer: s.repo, Activity: s.activity})
}
func (s *fakeStore) Repos() customer.Repos {
	return customer.Repos{Customer: s.repo, Activity: s.activity}
}

func actorContext(orgID, membershipID uuid.UUID, role tenant.Role) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: role}
}

func ownerActor() (tenant.Context, uuid.UUID) {
	org := uuid.Must(uuid.NewV7())
	return actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner), org
}

// --- tests ---

func TestUnit_Convert_EmployeeForbidden(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	org := uuid.Must(uuid.NewV7())
	leadID := uuid.Must(uuid.NewV7())
	store.repo.leadStatus[leadID] = "won"

	employeeCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	_, err := u.Convert(context.Background(), employeeCtx, leadID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden, got: %v", err)
	}
}

func TestUnit_Convert_ManagerForbidden(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	org := uuid.Must(uuid.NewV7())
	leadID := uuid.Must(uuid.NewV7())
	store.repo.leadStatus[leadID] = "won"

	managerCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleManager)
	_, err := u.Convert(context.Background(), managerCtx, leadID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for manager convert, got: %v", err)
	}
}

func TestUnit_Convert_Won_Succeeds_RecordsActivity(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	actor, _ := ownerActor()
	leadID := uuid.Must(uuid.NewV7())
	store.repo.leadStatus[leadID] = "won"

	created, err := u.Convert(context.Background(), actor, leadID)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if created.ConvertedFromLeadID != leadID {
		t.Errorf("expected converted_from_lead_id %s, got %s", leadID, created.ConvertedFromLeadID)
	}

	if len(store.activity.calls) != 1 || store.activity.calls[0].activityType != "lead_converted" {
		t.Fatalf("expected 1 lead_converted activity, got %+v", store.activity.calls)
	}
	if store.activity.calls[0].metadata["customer_id"] != created.ID {
		t.Errorf("expected metadata.customer_id = %s, got %v", created.ID, store.activity.calls[0].metadata)
	}
}

func TestUnit_Convert_NotWon_Returns422(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	actor, _ := ownerActor()
	leadID := uuid.Must(uuid.NewV7())
	store.repo.leadStatus[leadID] = "new"

	_, err := u.Convert(context.Background(), actor, leadID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_status_transition" {
		t.Fatalf("expected invalid_status_transition, got: %v", err)
	}
}

func TestUnit_Convert_Twice_Returns409(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	actor, _ := ownerActor()
	leadID := uuid.Must(uuid.NewV7())
	store.repo.leadStatus[leadID] = "won"

	if _, err := u.Convert(context.Background(), actor, leadID); err != nil {
		t.Fatalf("first convert: %v", err)
	}

	_, err := u.Convert(context.Background(), actor, leadID)
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "lead_already_converted" {
		t.Fatalf("expected lead_already_converted, got: %v", err)
	}
}

func TestUnit_Convert_LeadNotFound_Returns404(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	actor, _ := ownerActor()

	_, err := u.Convert(context.Background(), actor, uuid.Must(uuid.NewV7()))
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestUnit_Update_ManagerForbidden(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	actor, org := ownerActor()
	leadID := uuid.Must(uuid.NewV7())
	store.repo.leadStatus[leadID] = "won"
	created, err := u.Convert(context.Background(), actor, leadID)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	newName := "New Name"
	managerCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleManager)
	_, err = u.Update(context.Background(), managerCtx, created.ID, customer.UpdateCustomerInput{Name: &newName})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for manager update, got: %v", err)
	}
}

func TestUnit_Delete_ManagerForbidden(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	actor, org := ownerActor()
	leadID := uuid.Must(uuid.NewV7())
	store.repo.leadStatus[leadID] = "won"
	created, err := u.Convert(context.Background(), actor, leadID)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	managerCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleManager)
	err = u.Delete(context.Background(), managerCtx, created.ID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for manager delete, got: %v", err)
	}
}

func TestUnit_Get_EmployeeAllowed(t *testing.T) {
	store := newFakeStore()
	u := customer.NewUsecase(store)
	actor, org := ownerActor()
	leadID := uuid.Must(uuid.NewV7())
	store.repo.leadStatus[leadID] = "won"
	created, err := u.Convert(context.Background(), actor, leadID)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	employeeCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	// The fake repo doesn't enforce lead-based scoping (that's proven
	// against real Postgres in repository_test.go) — this only proves
	// the authz gate itself allows Employee through to the repository.
	if _, err := u.Get(context.Background(), employeeCtx, created.ID); err != nil {
		t.Fatalf("expected employee read to be authz-allowed, got: %v", err)
	}
}
