package form_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/form/... -run TestUnit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeFormRepo struct {
	byID map[uuid.UUID]*form.Form
}

func newFakeFormRepo() *fakeFormRepo {
	return &fakeFormRepo{byID: map[uuid.UUID]*form.Form{}}
}

func (f *fakeFormRepo) Create(_ context.Context, t tenant.Context, frm *form.Form) error {
	frm.OrganizationID = t.OrganizationID
	f.byID[frm.ID] = frm
	return nil
}

func (f *fakeFormRepo) FindByOrg(_ context.Context, t tenant.Context) ([]*form.Form, error) {
	var out []*form.Form
	for _, frm := range f.byID {
		if frm.OrganizationID == t.OrganizationID && frm.DeletedAt == nil {
			out = append(out, frm)
		}
	}
	return out, nil
}

func (f *fakeFormRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*form.Form, error) {
	frm, ok := f.byID[id]
	if !ok || frm.OrganizationID != t.OrganizationID || frm.DeletedAt != nil {
		return nil, httpx.ErrNotFound
	}
	return frm, nil
}

func (f *fakeFormRepo) Update(_ context.Context, t tenant.Context, id uuid.UUID, in form.UpdateInput) (*form.Form, error) {
	frm, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		frm.Name = *in.Name
	}
	if in.Fields != nil {
		frm.Fields = *in.Fields
	}
	if in.AllowedOrigins != nil {
		frm.AllowedOrigins = *in.AllowedOrigins
	}
	return frm, nil
}

func (f *fakeFormRepo) Delete(_ context.Context, t tenant.Context, id uuid.UUID) error {
	frm, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil // idempotent: already gone/deleted is not an error
	}
	now := frm.CreatedAt
	frm.DeletedAt = &now
	return nil
}

func (f *fakeFormRepo) FindByPublicKey(_ context.Context, publicKey string) (*form.Form, error) {
	for _, frm := range f.byID {
		if frm.PublicKey == publicKey && frm.DeletedAt == nil {
			return frm, nil
		}
	}
	return nil, httpx.ErrNotFound
}

type recordedAudit struct{ action string }

type fakeAuditRepo struct{ calls []recordedAudit }

func (f *fakeAuditRepo) Record(_ context.Context, _ tenant.Context, _ *uuid.UUID, action string) error {
	f.calls = append(f.calls, recordedAudit{action})
	return nil
}

type fakeStore struct {
	repo  *fakeFormRepo
	audit *fakeAuditRepo
}

func newFakeStore() *fakeStore {
	return &fakeStore{repo: newFakeFormRepo(), audit: &fakeAuditRepo{}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(form.Repos) error) error {
	return fn(form.Repos{Form: s.repo, Audit: s.audit})
}
func (s *fakeStore) Repos() form.Repos {
	return form.Repos{Form: s.repo, Audit: s.audit}
}

func actorContext(orgID, membershipID uuid.UUID, role tenant.Role) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: role}
}

func actorForRole(role tenant.Role) tenant.Context {
	org := uuid.Must(uuid.NewV7())
	return actorContext(org, uuid.Must(uuid.NewV7()), role)
}

func ownerActor() (tenant.Context, uuid.UUID) {
	org := uuid.Must(uuid.NewV7())
	return actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner), org
}

// --- Create: authz per role ---

func TestUnit_Create_OwnerAndAdminAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin} {
		store := newFakeStore()
		u := form.NewUsecase(store)
		actor := actorForRole(role)

		f, err := u.Create(context.Background(), actor, form.CreateInput{Name: "Website"})
		if err != nil {
			t.Fatalf("role %s: expected create to succeed, got: %v", role, err)
		}
		if f.Name != "Website" {
			t.Errorf("role %s: expected name %q, got %q", role, "Website", f.Name)
		}
		if f.PublicKey == "" {
			t.Errorf("role %s: expected a non-empty public_key", role)
		}
	}
}

func TestUnit_Create_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		store := newFakeStore()
		u := form.NewUsecase(store)
		actor := actorForRole(role)

		_, err := u.Create(context.Background(), actor, form.CreateInput{Name: "Website"})
		var derr *httpx.DomainError
		if !errors.As(err, &derr) || derr.Code != "forbidden" {
			t.Fatalf("role %s: expected forbidden, got: %v", role, err)
		}
	}
}

func TestUnit_Create_NameRequired(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	actor, _ := ownerActor()

	_, err := u.Create(context.Background(), actor, form.CreateInput{})
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected validation error for missing name, got: %v", err)
	}
}

func TestUnit_Create_NilFieldsDefaultsToDefaultFields(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	actor, _ := ownerActor()

	f, err := u.Create(context.Background(), actor, form.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(f.Fields) != len(form.AllFieldKeys) {
		t.Errorf("expected DefaultFields' key count, got %d keys", len(f.Fields))
	}
}

func TestUnit_Create_InvalidFieldsRejected(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	actor, _ := ownerActor()

	bad := form.Fields{form.FieldEmail: {Enabled: false, Required: true, Label: "Email"}}
	_, err := u.Create(context.Background(), actor, form.CreateInput{Name: "Website", Fields: &bad})
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected validation error for invalid fields, got: %v", err)
	}
}

func TestUnit_Create_RecordsAudit(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	actor, _ := ownerActor()

	if _, err := u.Create(context.Background(), actor, form.CreateInput{Name: "Website"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(store.audit.calls) != 1 || store.audit.calls[0].action != "form.created" {
		t.Fatalf("expected 1 form.created audit entry, got %+v", store.audit.calls)
	}
}

// --- List: authz per role ---

func TestUnit_List_OwnerAndAdminAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin} {
		store := newFakeStore()
		u := form.NewUsecase(store)
		actor := actorForRole(role)

		if _, err := u.List(context.Background(), actor); err != nil {
			t.Fatalf("role %s: expected list to succeed, got: %v", role, err)
		}
	}
}

func TestUnit_List_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		store := newFakeStore()
		u := form.NewUsecase(store)
		actor := actorForRole(role)

		_, err := u.List(context.Background(), actor)
		var derr *httpx.DomainError
		if !errors.As(err, &derr) || derr.Code != "forbidden" {
			t.Fatalf("role %s: expected forbidden, got: %v", role, err)
		}
	}
}

// --- Get: cross-org 404 ---

func TestUnit_Get_CrossOrg_Returns404(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	ownerA, _ := ownerActor()
	ownerB, _ := ownerActor()

	f, err := u.Create(context.Background(), ownerA, form.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = u.Get(context.Background(), ownerB, f.ID)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for cross-org get, got: %v", err)
	}
}

// --- Update: authz, validation, cross-org ---

func TestUnit_Update_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		store := newFakeStore()
		u := form.NewUsecase(store)
		org := uuid.Must(uuid.NewV7())
		owner := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
		f, err := u.Create(context.Background(), owner, form.CreateInput{Name: "Website"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		actor := actorContext(org, uuid.Must(uuid.NewV7()), role)
		newName := "Renamed"
		_, err = u.Update(context.Background(), actor, f.ID, form.UpdateInput{Name: &newName})
		var derr *httpx.DomainError
		if !errors.As(err, &derr) || derr.Code != "forbidden" {
			t.Fatalf("role %s: expected forbidden, got: %v", role, err)
		}
	}
}

func TestUnit_Update_InvalidFieldsRejected(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	actor, _ := ownerActor()
	f, err := u.Create(context.Background(), actor, form.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	bad := form.Fields{form.FieldEmail: {Enabled: false, Required: true, Label: "Email"}}
	_, err = u.Update(context.Background(), actor, f.ID, form.UpdateInput{Fields: &bad})
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected validation error for invalid fields, got: %v", err)
	}
}

func TestUnit_Update_CrossOrg_Returns404(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	ownerA, _ := ownerActor()
	ownerB, _ := ownerActor()

	f, err := u.Create(context.Background(), ownerA, form.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "Renamed"
	_, err = u.Update(context.Background(), ownerB, f.ID, form.UpdateInput{Name: &newName})
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for cross-org update, got: %v", err)
	}
}

// --- Delete: authz per role, not found, idempotency ---

func TestUnit_Delete_OwnerAndAdminAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin} {
		store := newFakeStore()
		u := form.NewUsecase(store)
		org := uuid.Must(uuid.NewV7())
		owner := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
		f, err := u.Create(context.Background(), owner, form.CreateInput{Name: "Website"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		actor := actorContext(org, uuid.Must(uuid.NewV7()), role)
		if err := u.Delete(context.Background(), actor, f.ID); err != nil {
			t.Fatalf("role %s: expected delete to succeed, got: %v", role, err)
		}
	}
}

func TestUnit_Delete_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		store := newFakeStore()
		u := form.NewUsecase(store)
		org := uuid.Must(uuid.NewV7())
		owner := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
		f, err := u.Create(context.Background(), owner, form.CreateInput{Name: "Website"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		actor := actorContext(org, uuid.Must(uuid.NewV7()), role)
		err = u.Delete(context.Background(), actor, f.ID)
		var derr *httpx.DomainError
		if !errors.As(err, &derr) || derr.Code != "forbidden" {
			t.Fatalf("role %s: expected forbidden, got: %v", role, err)
		}
	}
}

func TestUnit_Delete_NotFound_Returns404(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	actor, _ := ownerActor()

	err := u.Delete(context.Background(), actor, uuid.Must(uuid.NewV7()))
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestUnit_Delete_CrossOrg_Returns404(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	ownerA, _ := ownerActor()
	ownerB, _ := ownerActor()

	f, err := u.Create(context.Background(), ownerA, form.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = u.Delete(context.Background(), ownerB, f.ID)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for cross-org delete, got: %v", err)
	}
}

// TestUnit_Delete_Twice_SecondCallIs404 proves forms do NOT share
// apikey.Revoke's idempotent-success shape: a form is soft-deleted, so
// once deleted_at is set the same FindByID check that returns 404 for
// missing/cross-org (Rule #6) returns 404 for "already deleted" too —
// the second Delete call never even reaches the repository UPDATE.
func TestUnit_Delete_Twice_SecondCallIs404(t *testing.T) {
	store := newFakeStore()
	u := form.NewUsecase(store)
	actor, _ := ownerActor()

	f, err := u.Create(context.Background(), actor, form.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := u.Delete(context.Background(), actor, f.ID); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	// A second delete finds nothing (FindByID excludes deleted rows), so
	// it must answer 404 — this mirrors the repository's own contract
	// (Usecase.Delete calls FindByID first), unlike apikey's Revoke
	// where a revoked-but-not-deleted row is still findable. Forms are
	// soft-deleted, not merely flagged, so "already deleted" and "never
	// existed" are the same 404 here.
	if err := u.Delete(context.Background(), actor, f.ID); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("second delete: expected httpx.ErrNotFound, got: %v", err)
	}
}
