package form_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/form/... -run TestUnit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/captcha"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/formtoken"
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

func (f *fakeFormRepo) IncrementSubmitCount(_ context.Context, t tenant.Context, id uuid.UUID) error {
	frm, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return err
	}
	frm.SubmitCount++
	return nil
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

// testFormTokenSecret satisfies FORM_TOKEN_SECRET's real min-length
// requirement (config.go) so tests that aren't specifically exercising
// the time-trap don't fail on it incidentally.
var testFormTokenSecret = []byte("test-form-token-secret-at-least-32-bytes")

type recordedLeadCreate struct {
	t                            tenant.Context
	name                         string
	email, phone, company, notes *string
	rawPayload                   []byte
}

// fakeLeadCreator satisfies form.LeadCreator (port.go) — err lets a
// test simulate authz.Require's own rejection (the real path routes
// through it INSIDE lead.Usecase.Create; this fake stands in for that
// whole call, not just its authz gate) or any other failure the real
// bridge could surface.
type fakeLeadCreator struct {
	calls []recordedLeadCreate
	err   error
}

func (f *fakeLeadCreator) CreateFromForm(_ context.Context, t tenant.Context, name string, email, phone, company, notes *string, rawPayload []byte) (uuid.UUID, error) {
	f.calls = append(f.calls, recordedLeadCreate{t, name, email, phone, company, notes, rawPayload})
	if f.err != nil {
		return uuid.Nil, f.err
	}
	return uuid.Must(uuid.NewV7()), nil
}

// newTestUsecase supplies sensible always-succeeding defaults for the
// three dependencies #87 added to NewUsecase (captcha.NoopVerifier,
// testFormTokenSecret, an empty-but-successful fakeLeadCreator) — used
// by every test in this file that exercises CRUD, not Submit, where
// these three are irrelevant plumbing rather than what's under test.
// Submit-specific tests below construct form.NewUsecase directly so
// each can swap in the ONE fake it actually needs control over.
func newTestUsecase(store form.Store) *form.Usecase {
	return form.NewUsecase(store, captcha.NoopVerifier{}, testFormTokenSecret, &fakeLeadCreator{})
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
		u := newTestUsecase(store)
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
		u := newTestUsecase(store)
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
	u := newTestUsecase(store)
	actor, _ := ownerActor()

	_, err := u.Create(context.Background(), actor, form.CreateInput{})
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected validation error for missing name, got: %v", err)
	}
}

func TestUnit_Create_NilFieldsDefaultsToDefaultFields(t *testing.T) {
	store := newFakeStore()
	u := newTestUsecase(store)
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
	u := newTestUsecase(store)
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
	u := newTestUsecase(store)
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
		u := newTestUsecase(store)
		actor := actorForRole(role)

		if _, err := u.List(context.Background(), actor); err != nil {
			t.Fatalf("role %s: expected list to succeed, got: %v", role, err)
		}
	}
}

func TestUnit_List_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		store := newFakeStore()
		u := newTestUsecase(store)
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
	u := newTestUsecase(store)
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
		u := newTestUsecase(store)
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
	u := newTestUsecase(store)
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
	u := newTestUsecase(store)
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
		u := newTestUsecase(store)
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
		u := newTestUsecase(store)
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
	u := newTestUsecase(store)
	actor, _ := ownerActor()

	err := u.Delete(context.Background(), actor, uuid.Must(uuid.NewV7()))
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestUnit_Delete_CrossOrg_Returns404(t *testing.T) {
	store := newFakeStore()
	u := newTestUsecase(store)
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
	u := newTestUsecase(store)
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

// --- Submit (Phase 6 #87) ---

// fakeCaptchaVerifier lets a test choose success/failure directly,
// unlike captcha.NoopVerifier (always succeeds) — used only by the
// captcha-specific tests below; every other Submit test uses
// newTestUsecase's NoopVerifier default.
type fakeCaptchaVerifier struct{ err error }

func (f fakeCaptchaVerifier) Verify(_ context.Context, _, _ string) error { return f.err }

// seedForm inserts a form directly into the fake repo, bypassing
// Usecase.Create — Submit tests need precise control over
// AllowedOrigins/Fields/ID that going through Create's own defaulting
// would obscure.
func seedForm(store *fakeStore, org uuid.UUID, publicKey string, allowedOrigins []string, fields form.Fields) *form.Form {
	f := &form.Form{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: org, PublicKey: publicKey,
		Name: "Test Form", Fields: fields, AllowedOrigins: allowedOrigins,
	}
	store.repo.byID[f.ID] = f
	return f
}

func validSubmitInput() form.SubmitInput {
	return form.SubmitInput{
		Name:   "Budi Santoso",
		Origin: "https://example.com",
	}
}

func TestUnit_Submit_UnknownPublicKey_Returns404(t *testing.T) {
	store := newFakeStore()
	u := newTestUsecase(store)

	_, err := u.Submit(context.Background(), "pk_does-not-exist", validSubmitInput())
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestUnit_Submit_DeletedForm_Returns404(t *testing.T) {
	store := newFakeStore()
	u := newTestUsecase(store)
	org := uuid.Must(uuid.NewV7())
	f := seedForm(store, org, "pk_deleted", []string{"https://example.com"}, form.DefaultFields())
	now := time.Now()
	f.DeletedAt = &now

	_, err := u.Submit(context.Background(), "pk_deleted", validSubmitInput())
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a deleted form, got: %v", err)
	}
}

func TestUnit_Submit_OriginNotInAllowlist_Returns403(t *testing.T) {
	store := newFakeStore()
	u := newTestUsecase(store)
	org := uuid.Must(uuid.NewV7())
	seedForm(store, org, "pk_test", []string{"https://allowed.example.com"}, form.DefaultFields())

	in := validSubmitInput()
	in.Origin = "https://attacker.example.com"
	_, err := u.Submit(context.Background(), "pk_test", in)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "origin_not_allowed" {
		t.Fatalf("expected origin_not_allowed, got: %v", err)
	}
}

// TestUnit_Submit_EmptyAllowlist_AlwaysRejects is TD §7's "gagal
// tertutup, bukan terbuka" reasoning extended from the embed page's
// frame-ancestors to submission itself — a form nobody has configured
// an origin for yet shouldn't accept submissions from anywhere.
func TestUnit_Submit_EmptyAllowlist_AlwaysRejects(t *testing.T) {
	store := newFakeStore()
	u := newTestUsecase(store)
	org := uuid.Must(uuid.NewV7())
	seedForm(store, org, "pk_test", []string{}, form.DefaultFields())

	in := validSubmitInput()
	in.Origin = "https://example.com"
	_, err := u.Submit(context.Background(), "pk_test", in)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "origin_not_allowed" {
		t.Fatalf("expected origin_not_allowed for an empty allowlist, got: %v", err)
	}
}

func TestUnit_Submit_MissingOriginHeader_Returns403(t *testing.T) {
	store := newFakeStore()
	u := newTestUsecase(store)
	org := uuid.Must(uuid.NewV7())
	seedForm(store, org, "pk_test", []string{"https://example.com"}, form.DefaultFields())

	in := validSubmitInput()
	in.Origin = "" // no Origin header sent at all
	_, err := u.Submit(context.Background(), "pk_test", in)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "origin_not_allowed" {
		t.Fatalf("expected origin_not_allowed for a missing Origin header, got: %v", err)
	}
}

// TestUnit_Submit_HoneypotFilled_FakeSuccessWithoutCreatingLead is #87's
// direct acceptance criterion: honeypot terisi → respons TIDAK BISA
// DIBEDAKAN dari sukses, tidak ada lead. Proven here at the usecase
// level: Submit returns (id, nil) exactly like a real success, but
// leadCreator (the only path to persistence) is never called.
func TestUnit_Submit_HoneypotFilled_FakeSuccessWithoutCreatingLead(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	seedForm(store, org, "pk_test", []string{"https://example.com"}, form.DefaultFields())
	creator := &fakeLeadCreator{}
	u := form.NewUsecase(store, captcha.NoopVerifier{}, testFormTokenSecret, creator)

	in := validSubmitInput()
	in.HoneypotFilled = true
	// Deliberately garbage — honeypot must short-circuit BEFORE the
	// time-trap check even looks at this (TD §5: honeypot at position 6,
	// before position 7). If this test passed only because the token
	// happened to validate, it wouldn't be proving what it claims to.
	in.FormToken = "not-a-real-token-at-all"

	id, err := u.Submit(context.Background(), "pk_test", in)
	if err != nil {
		t.Fatalf("expected fake success (nil error) for a honeypot-filled submission, got: %v", err)
	}
	if id == uuid.Nil {
		t.Error("expected a real-looking (non-nil) id even for the fake-success path — see honeypotFakeLeadID's doc comment")
	}
	if len(creator.calls) != 0 {
		t.Errorf("expected leadCreator to never be called for a honeypot-filled submission, got %d calls", len(creator.calls))
	}
}

// TestUnit_Submit_HoneypotChecked_BeforeOriginPasses proves honeypot
// truly sits at TD §5's position 6, AFTER origin (position 5) — an
// origin rejection must win even when the honeypot is also filled,
// since step 5 runs first and returns before step 6 is ever reached.
func TestUnit_Submit_HoneypotChecked_AfterOrigin(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	seedForm(store, org, "pk_test", []string{"https://allowed.example.com"}, form.DefaultFields())
	u := newTestUsecase(store)

	in := validSubmitInput()
	in.Origin = "https://attacker.example.com"
	in.HoneypotFilled = true

	_, err := u.Submit(context.Background(), "pk_test", in)
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "origin_not_allowed" {
		t.Fatalf("expected origin_not_allowed to win over the honeypot fake-success, got: %v", err)
	}
}

func TestUnit_Submit_InvalidFormToken_Returns400(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	seedForm(store, org, "pk_test", []string{"https://example.com"}, form.DefaultFields())
	u := newTestUsecase(store)

	in := validSubmitInput()
	in.FormToken = "garbage"
	_, err := u.Submit(context.Background(), "pk_test", in)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "form_token_invalid" {
		t.Fatalf("expected form_token_invalid, got: %v", err)
	}
}

// TestUnit_Submit_FormTokenForAnotherForm_Rejected is #87's own
// acceptance criterion, verbatim: "token form lain ditolak".
func TestUnit_Submit_FormTokenForAnotherForm_Rejected(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	target := seedForm(store, org, "pk_target", []string{"https://example.com"}, form.DefaultFields())
	other := seedForm(store, org, "pk_other", []string{"https://example.com"}, form.DefaultFields())
	_ = target
	u := newTestUsecase(store)

	// A genuinely valid, well-timed token — just issued for the WRONG
	// form.
	tokenForOtherForm := formtoken.Issue(testFormTokenSecret, other.ID)
	time.Sleep(2100 * time.Millisecond) // clear the 2s minimum age

	in := validSubmitInput()
	in.FormToken = tokenForOtherForm
	_, err := u.Submit(context.Background(), "pk_target", in)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "form_token_invalid" {
		t.Fatalf("expected form_token_invalid for a token issued to a different form, got: %v", err)
	}
}

func TestUnit_Submit_CaptchaFails_Returns400(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	f := seedForm(store, org, "pk_test", []string{"https://example.com"}, form.DefaultFields())
	u := form.NewUsecase(store, fakeCaptchaVerifier{err: captcha.ErrCaptchaFailed}, testFormTokenSecret, &fakeLeadCreator{})

	in := validSubmitInput()
	in.FormToken = validTokenFor(f.ID)
	_, err := u.Submit(context.Background(), "pk_test", in)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "captcha_failed" {
		t.Fatalf("expected captcha_failed, got: %v", err)
	}
}

func TestUnit_Submit_RequiredFieldMissing_ReturnsValidationError(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	// DefaultFields() marks name and phone Required — omitting phone
	// must fail validation.
	f := seedForm(store, org, "pk_test", []string{"https://example.com"}, form.DefaultFields())
	u := newTestUsecase(store)

	in := validSubmitInput()
	in.FormToken = validTokenFor(f.ID)
	in.Phone = nil
	_, err := u.Submit(context.Background(), "pk_test", in)

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected a validation error for a missing required field, got: %v", err)
	}
}

// TestUnit_Submit_RequiredProductField_ValidatedEvenWithoutLeadColumn
// proves validateRequiredFields checks "product" — the one field with
// no dedicated Lead column at all (entity.go's Fields doc comment) —
// exactly like any other required field.
func TestUnit_Submit_RequiredProductField_ValidatedEvenWithoutLeadColumn(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	fields := form.DefaultFields()
	fields[form.FieldProduct] = form.FieldConfig{Enabled: true, Required: true, Label: "Layanan"}
	f := seedForm(store, org, "pk_test", []string{"https://example.com"}, fields)
	u := newTestUsecase(store)

	in := validSubmitInput()
	in.FormToken = validTokenFor(f.ID)
	in.Phone = ptrStr("0812")
	// Product deliberately left nil.
	_, err := u.Submit(context.Background(), "pk_test", in)

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected a validation error for a missing required product field, got: %v", err)
	}
}

func TestUnit_Submit_Success_CreatesLeadAndIncrementsSubmitCount(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	f := seedForm(store, org, "pk_test", []string{"https://example.com"}, form.DefaultFields())
	creator := &fakeLeadCreator{}
	u := form.NewUsecase(store, captcha.NoopVerifier{}, testFormTokenSecret, creator)

	in := form.SubmitInput{
		Name: "Budi Santoso", Phone: ptrStr("0812xxxx"), Origin: "https://example.com",
		FormToken: validTokenFor(f.ID), RawPayload: []byte(`{"name":"Budi Santoso"}`),
	}
	id, err := u.Submit(context.Background(), "pk_test", in)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if id == uuid.Nil {
		t.Error("expected a non-nil lead id")
	}
	if len(creator.calls) != 1 {
		t.Fatalf("expected exactly 1 leadCreator call, got %d", len(creator.calls))
	}
	call := creator.calls[0]
	if call.t.PrincipalType != tenant.PrincipalPublicForm {
		t.Errorf("expected PrincipalType public_form, got %q", call.t.PrincipalType)
	}
	if call.t.FormID == nil || *call.t.FormID != f.ID {
		t.Errorf("expected FormID %s, got %v", f.ID, call.t.FormID)
	}
	if call.t.OrganizationID != org {
		t.Errorf("expected OrganizationID %s, got %s", org, call.t.OrganizationID)
	}
	if call.name != "Budi Santoso" {
		t.Errorf("expected name passed through, got %q", call.name)
	}

	updated, err := store.repo.FindByPublicKey(context.Background(), "pk_test")
	if err != nil {
		t.Fatalf("find by public key: %v", err)
	}
	if updated.SubmitCount != 1 {
		t.Errorf("expected submit_count incremented to 1, got %d", updated.SubmitCount)
	}
}

func TestUnit_Submit_LeadCreatorFails_PropagatesErrorAndSkipsIncrement(t *testing.T) {
	store := newFakeStore()
	org := uuid.Must(uuid.NewV7())
	f := seedForm(store, org, "pk_test", []string{"https://example.com"}, form.DefaultFields())
	wantErr := errors.New("boom")
	creator := &fakeLeadCreator{err: wantErr}
	u := form.NewUsecase(store, captcha.NoopVerifier{}, testFormTokenSecret, creator)

	in := validSubmitInput()
	in.Phone = ptrStr("0812")
	in.FormToken = validTokenFor(f.ID)
	_, err := u.Submit(context.Background(), "pk_test", in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected leadCreator's error to propagate, got: %v", err)
	}

	updated, findErr := store.repo.FindByPublicKey(context.Background(), "pk_test")
	if findErr != nil {
		t.Fatalf("find by public key: %v", findErr)
	}
	if updated.SubmitCount != 0 {
		t.Errorf("expected submit_count to stay 0 when lead creation fails, got %d", updated.SubmitCount)
	}
}

// validTokenFor mints a token that's already past formtoken's 2-second
// minimum age by sleeping — every Submit test that needs to reach past
// the time-trap check uses this rather than inlining the sleep,
// documented once here instead of at every call site.
func validTokenFor(formID uuid.UUID) string {
	token := formtoken.Issue(testFormTokenSecret, formID)
	time.Sleep(2100 * time.Millisecond)
	return token
}

func ptrStr(s string) *string { return &s }
