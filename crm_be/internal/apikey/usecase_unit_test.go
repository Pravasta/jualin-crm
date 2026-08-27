package apikey_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/apikey/... -run TestUnit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/apikey"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeAPIKeyRepo struct {
	byID map[uuid.UUID]*apikey.APIKey
}

func newFakeAPIKeyRepo() *fakeAPIKeyRepo {
	return &fakeAPIKeyRepo{byID: map[uuid.UUID]*apikey.APIKey{}}
}

func (f *fakeAPIKeyRepo) Create(_ context.Context, t tenant.Context, k *apikey.APIKey) error {
	k.OrganizationID = t.OrganizationID
	f.byID[k.ID] = k
	return nil
}

func (f *fakeAPIKeyRepo) FindByOrg(_ context.Context, t tenant.Context) ([]*apikey.APIKey, error) {
	var out []*apikey.APIKey
	for _, k := range f.byID {
		if k.OrganizationID == t.OrganizationID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeAPIKeyRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*apikey.APIKey, error) {
	k, ok := f.byID[id]
	if !ok || k.OrganizationID != t.OrganizationID {
		return nil, httpx.ErrNotFound
	}
	return k, nil
}

func (f *fakeAPIKeyRepo) Revoke(_ context.Context, t tenant.Context, id uuid.UUID) error {
	k, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return err
	}
	if k.RevokedAt == nil {
		now := k.CreatedAt
		k.RevokedAt = &now
	}
	return nil
}

func (f *fakeAPIKeyRepo) FindByKeyID(_ context.Context, keyID string) (*apikey.APIKey, error) {
	for _, k := range f.byID {
		if k.KeyID == keyID {
			return k, nil
		}
	}
	return nil, httpx.ErrNotFound
}

type recordedAudit struct {
	action string
}

type fakeAuditRepo struct{ calls []recordedAudit }

func (f *fakeAuditRepo) Record(_ context.Context, _ tenant.Context, _ *uuid.UUID, action string) error {
	f.calls = append(f.calls, recordedAudit{action})
	return nil
}

type fakeStore struct {
	repo  *fakeAPIKeyRepo
	audit *fakeAuditRepo
}

func newFakeStore() *fakeStore {
	return &fakeStore{repo: newFakeAPIKeyRepo(), audit: &fakeAuditRepo{}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(apikey.Repos) error) error {
	return fn(apikey.Repos{APIKey: s.repo, Audit: s.audit})
}
func (s *fakeStore) Repos() apikey.Repos {
	return apikey.Repos{APIKey: s.repo, Audit: s.audit}
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
		u := apikey.NewUsecase(store)
		actor := actorForRole(role)

		k, raw, err := u.Create(context.Background(), actor, apikey.CreateInput{Name: "Website"})
		if err != nil {
			t.Fatalf("role %s: expected create to succeed, got: %v", role, err)
		}
		if k.Name != "Website" {
			t.Errorf("role %s: expected name %q, got %q", role, "Website", k.Name)
		}
		if raw == "" {
			t.Errorf("role %s: expected a non-empty raw credential", role)
		}
	}
}

func TestUnit_Create_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		store := newFakeStore()
		u := apikey.NewUsecase(store)
		actor := actorForRole(role)

		_, _, err := u.Create(context.Background(), actor, apikey.CreateInput{Name: "Website"})
		var derr *httpx.DomainError
		if !errors.As(err, &derr) || derr.Code != "forbidden" {
			t.Fatalf("role %s: expected forbidden, got: %v", role, err)
		}
	}
}

func TestUnit_Create_NameRequired(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	actor, _ := ownerActor()

	_, _, err := u.Create(context.Background(), actor, apikey.CreateInput{})
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected validation error for missing name, got: %v", err)
	}
}

func TestUnit_Create_UnknownScopeRejected(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	actor, _ := ownerActor()

	_, _, err := u.Create(context.Background(), actor, apikey.CreateInput{Name: "Website", Scopes: []string{"leads:read"}})
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected validation error for unknown scope, got: %v", err)
	}
}

func TestUnit_Create_EmptyScopesDefaultsToLeadsWrite(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	actor, _ := ownerActor()

	k, _, err := u.Create(context.Background(), actor, apikey.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(k.Scopes) != 1 || k.Scopes[0] != apikey.ScopeLeadsWrite {
		t.Errorf("expected default scopes [%q], got %v", apikey.ScopeLeadsWrite, k.Scopes)
	}
}

func TestUnit_Create_RecordsAudit(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	actor, _ := ownerActor()

	if _, _, err := u.Create(context.Background(), actor, apikey.CreateInput{Name: "Website"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(store.audit.calls) != 1 || store.audit.calls[0].action != "api_key.created" {
		t.Fatalf("expected 1 api_key.created audit entry, got %+v", store.audit.calls)
	}
}

// --- List: authz per role ---

func TestUnit_List_OwnerAndAdminAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin} {
		store := newFakeStore()
		u := apikey.NewUsecase(store)
		actor := actorForRole(role)

		if _, err := u.List(context.Background(), actor); err != nil {
			t.Fatalf("role %s: expected list to succeed, got: %v", role, err)
		}
	}
}

func TestUnit_List_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		store := newFakeStore()
		u := apikey.NewUsecase(store)
		actor := actorForRole(role)

		_, err := u.List(context.Background(), actor)
		var derr *httpx.DomainError
		if !errors.As(err, &derr) || derr.Code != "forbidden" {
			t.Fatalf("role %s: expected forbidden, got: %v", role, err)
		}
	}
}

// --- Revoke: authz per role, not found, idempotency ---

func TestUnit_Revoke_OwnerAndAdminAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin} {
		store := newFakeStore()
		u := apikey.NewUsecase(store)
		org := uuid.Must(uuid.NewV7())
		owner := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
		k, _, err := u.Create(context.Background(), owner, apikey.CreateInput{Name: "Website"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		actor := actorContext(org, uuid.Must(uuid.NewV7()), role)
		if err := u.Revoke(context.Background(), actor, k.ID); err != nil {
			t.Fatalf("role %s: expected revoke to succeed, got: %v", role, err)
		}
	}
}

func TestUnit_Revoke_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		store := newFakeStore()
		u := apikey.NewUsecase(store)
		org := uuid.Must(uuid.NewV7())
		owner := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
		k, _, err := u.Create(context.Background(), owner, apikey.CreateInput{Name: "Website"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		actor := actorContext(org, uuid.Must(uuid.NewV7()), role)
		err = u.Revoke(context.Background(), actor, k.ID)
		var derr *httpx.DomainError
		if !errors.As(err, &derr) || derr.Code != "forbidden" {
			t.Fatalf("role %s: expected forbidden, got: %v", role, err)
		}
	}
}

func TestUnit_Revoke_NotFound_Returns404(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	actor, _ := ownerActor()

	err := u.Revoke(context.Background(), actor, uuid.Must(uuid.NewV7()))
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestUnit_Revoke_CrossOrg_Returns404(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	ownerA, _ := ownerActor()
	ownerB, _ := ownerActor()

	k, _, err := u.Create(context.Background(), ownerA, apikey.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = u.Revoke(context.Background(), ownerB, k.ID)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for cross-org revoke, got: %v", err)
	}
}

// TestUnit_Revoke_Twice_StaysSuccessful proves the SECOND revoke of an
// already-revoked key still succeeds (TD §9: "revoke kedua tetap 204")
// rather than erroring — existence was already confirmed by the first
// call, so a second no-op UPDATE is not a failure.
func TestUnit_Revoke_Twice_StaysSuccessful(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	actor, _ := ownerActor()

	k, _, err := u.Create(context.Background(), actor, apikey.CreateInput{Name: "Website"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := u.Revoke(context.Background(), actor, k.ID); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	if err := u.Revoke(context.Background(), actor, k.ID); err != nil {
		t.Fatalf("second revoke: expected success (idempotent), got: %v", err)
	}

	revokedCount := 0
	for _, c := range store.audit.calls {
		if c.action == "api_key.revoked" {
			revokedCount++
		}
	}
	if revokedCount != 2 {
		t.Fatalf("expected 2 api_key.revoked audit entries (one per call, both succeeded), got %d in %+v", revokedCount, store.audit.calls)
	}
}
