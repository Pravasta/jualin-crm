package apikey_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/apikey/... -run TestUnit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/apikey"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeAPIKeyRepo struct {
	byID            map[uuid.UUID]*apikey.APIKey
	touchedLastUsed []uuid.UUID
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

func (f *fakeAPIKeyRepo) TouchLastUsed(_ context.Context, id uuid.UUID) error {
	f.touchedLastUsed = append(f.touchedLastUsed, id)
	if k, ok := f.byID[id]; ok {
		now := k.CreatedAt
		k.LastUsedAt = &now
	}
	return nil
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

// --- ResolveAPIKey ---

// testKeyID/testSecret mirror apikey's real 12/43-character format
// (entity_test.go's TestGenerate_ProducesExpectedLengths locks the real
// constants) without reaching into the package's unexported generate() —
// this file stays package apikey_test, so it only ever exercises
// ResolveAPIKey's public behavior, never its internals directly.
const testKeyID = "abcdefghijkl"
const testSecret = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 43 chars

func testSecretHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func seedResolvableKey(store *fakeStore, org uuid.UUID, scopes []string) *apikey.APIKey {
	k := &apikey.APIKey{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: org,
		KeyID: testKeyID, SecretHash: testSecretHash(testSecret), KeyPrefix: "jln_live_abcd",
		Name: "Website", Scopes: scopes, CreatedAt: time.Now(),
	}
	store.repo.byID[k.ID] = k
	return k
}

func TestUnit_ResolveAPIKey_Success(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	org := uuid.Must(uuid.NewV7())
	k := seedResolvableKey(store, org, []string{"leads:write"})

	got, err := u.ResolveAPIKey(context.Background(), "jln_live_"+testKeyID+"_"+testSecret)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.OrganizationID != org {
		t.Errorf("expected OrganizationID %s, got %s", org, got.OrganizationID)
	}
	if got.PrincipalType != tenant.PrincipalAPIKey {
		t.Errorf("expected PrincipalType api_key, got %q", got.PrincipalType)
	}
	if got.APIKeyID == nil || *got.APIKeyID != k.ID {
		t.Errorf("expected APIKeyID %s, got %v", k.ID, got.APIKeyID)
	}
	if len(got.Scopes) != 1 || got.Scopes[0] != "leads:write" {
		t.Errorf("expected scopes [leads:write], got %v", got.Scopes)
	}
	if got.MembershipID != nil || got.UserID != nil || got.Role != "" {
		t.Errorf("expected no person-identity fields set for an api_key context, got %+v", got)
	}
}

// TestUnit_ResolveAPIKey_EveryFailureReasonIsIdentical is the acceptance
// criterion verbatim: key_id unknown, secret wrong, and key revoked
// must produce the SAME 401 — distinguishing them would tell a guesser
// which key_id ever existed (Rule #6's reasoning, applied to key_id
// instead of a resource id).
func TestUnit_ResolveAPIKey_EveryFailureReasonIsIdentical(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	org := uuid.Must(uuid.NewV7())
	revoked := seedResolvableKey(store, org, []string{"leads:write"})
	now := time.Now()
	revoked.RevokedAt = &now

	valid := seedResolvableKey(store, org, []string{"leads:write"})
	_ = valid

	scenarios := map[string]string{
		"malformed":    "not-a-jln-credential-at-all",
		"unknown key":  "jln_live_zzzzzzzzzzzz_" + testSecret,
		"wrong secret": "jln_live_" + testKeyID + "_" + strings.Repeat("z", 43),
		"revoked key":  "jln_live_" + revoked.KeyID + "_" + testSecret,
	}

	var messages []string
	for name, raw := range scenarios {
		t.Run(name, func(t *testing.T) {
			_, err := u.ResolveAPIKey(context.Background(), raw)
			var derr *httpx.DomainError
			if !errors.As(err, &derr) || derr.Status != http.StatusUnauthorized || derr.Code != "invalid_api_key" {
				t.Fatalf("expected 401 invalid_api_key, got: %v", err)
			}
			messages = append(messages, derr.Message)
		})
	}
	for i := 1; i < len(messages); i++ {
		if messages[i] != messages[0] {
			t.Errorf("expected every failure message identical, got %q and %q", messages[0], messages[i])
		}
	}
}

// TestUnit_ResolveAPIKey_LastUsedThrottled proves the second resolve of
// the SAME key within the 5-minute window does not write last_used_at
// again (TD §10) — the fake repo records every TouchLastUsed call it
// receives, so this counts them directly rather than inferring the
// throttle from timing.
func TestUnit_ResolveAPIKey_LastUsedThrottled(t *testing.T) {
	store := newFakeStore()
	u := apikey.NewUsecase(store)
	org := uuid.Must(uuid.NewV7())
	seedResolvableKey(store, org, []string{"leads:write"})
	raw := "jln_live_" + testKeyID + "_" + testSecret

	if _, err := u.ResolveAPIKey(context.Background(), raw); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if _, err := u.ResolveAPIKey(context.Background(), raw); err != nil {
		t.Fatalf("second resolve: %v", err)
	}

	if len(store.repo.touchedLastUsed) != 1 {
		t.Fatalf("expected exactly 1 TouchLastUsed call across 2 resolves within the throttle window, got %d", len(store.repo.touchedLastUsed))
	}
}
