package webhook_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/webhook/... -run TestUnit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/safedial"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

// --- fakes ---

type fakeEndpointRepo struct {
	byID map[uuid.UUID]*webhook.Endpoint
}

func newFakeEndpointRepo() *fakeEndpointRepo {
	return &fakeEndpointRepo{byID: map[uuid.UUID]*webhook.Endpoint{}}
}

func (f *fakeEndpointRepo) Create(_ context.Context, t tenant.Context, e *webhook.Endpoint) error {
	e.OrganizationID = t.OrganizationID
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	f.byID[e.ID] = e
	return nil
}

func (f *fakeEndpointRepo) FindByOrg(_ context.Context, t tenant.Context) ([]*webhook.Endpoint, error) {
	var out []*webhook.Endpoint
	for _, e := range f.byID {
		if e.OrganizationID == t.OrganizationID && e.DeletedAt == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeEndpointRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*webhook.Endpoint, error) {
	e, ok := f.byID[id]
	if !ok || e.OrganizationID != t.OrganizationID || e.DeletedAt != nil {
		return nil, httpx.ErrNotFound
	}
	return e, nil
}

func (f *fakeEndpointRepo) Update(_ context.Context, t tenant.Context, id uuid.UUID, in webhook.UpdateInput) (*webhook.Endpoint, error) {
	e, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if in.URL != nil {
		e.URL = *in.URL
	}
	if in.Events != nil {
		e.Events = *in.Events
	}
	if in.Description != nil {
		e.Description = *in.Description
	}
	if in.IsActive != nil {
		e.IsActive = *in.IsActive
	}
	return e, nil
}

func (f *fakeEndpointRepo) Delete(_ context.Context, t tenant.Context, id uuid.UUID) error {
	e, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil
	}
	now := time.Now()
	e.DeletedAt = &now
	return nil
}

type fakeDeliveryRepo struct {
	byID map[uuid.UUID]*webhook.Delivery
}

func newFakeDeliveryRepo() *fakeDeliveryRepo {
	return &fakeDeliveryRepo{byID: map[uuid.UUID]*webhook.Delivery{}}
}

func (f *fakeDeliveryRepo) Enqueue(_ context.Context, t tenant.Context, d *webhook.Delivery) error {
	d.OrganizationID = t.OrganizationID
	d.Status = webhook.StatusPending
	f.byID[d.ID] = d
	return nil
}

func (f *fakeDeliveryRepo) FindByEndpoint(_ context.Context, t tenant.Context, endpointID uuid.UUID, _, _ int) ([]*webhook.Delivery, int, error) {
	var out []*webhook.Delivery
	for _, d := range f.byID {
		if d.OrganizationID == t.OrganizationID && d.EndpointID == endpointID {
			out = append(out, d)
		}
	}
	return out, len(out), nil
}

func (f *fakeDeliveryRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*webhook.Delivery, error) {
	d, ok := f.byID[id]
	if !ok || d.OrganizationID != t.OrganizationID {
		return nil, httpx.ErrNotFound
	}
	return d, nil
}

func (f *fakeDeliveryRepo) MarkForRetry(_ context.Context, t tenant.Context, id uuid.UUID) (*webhook.Delivery, error) {
	d, ok := f.byID[id]
	if !ok || d.OrganizationID != t.OrganizationID || d.Status != webhook.StatusFailed {
		return nil, webhook.ErrDeliveryNotRetryable
	}
	d.Status = webhook.StatusPending
	d.Attempt = 0
	return d, nil
}

func (f *fakeDeliveryRepo) ClaimDue(context.Context, int) ([]*webhook.Delivery, error) {
	return nil, nil
}
func (f *fakeDeliveryRepo) Reap(context.Context, time.Time) (int, error)  { return 0, nil }
func (f *fakeDeliveryRepo) Purge(context.Context, time.Time) (int, error) { return 0, nil }

type fakeAudit struct{ actions []string }

func (f *fakeAudit) Record(_ context.Context, _ tenant.Context, _ *uuid.UUID, action string) error {
	f.actions = append(f.actions, action)
	return nil
}

type fakeStore struct{ repos webhook.Repos }

func (f *fakeStore) InTx(_ context.Context, fn func(webhook.Repos) error) error { return fn(f.repos) }
func (f *fakeStore) Repos() webhook.Repos                                       { return f.repos }

// fakeURLValidator lets a test decide whether a URL passes without any
// DNS. err != nil is treated as safedial.ErrURLNotAllowed by wrapping.
type fakeURLValidator struct{ deny bool }

func (f *fakeURLValidator) ValidateURL(context.Context, string) error {
	if f.deny {
		return fmt.Errorf("%w: resolves to 10.0.0.1, in a denied range", safedial.ErrURLNotAllowed)
	}
	return nil
}

func testUsecase(deny bool) (*webhook.Usecase, *fakeStore, *fakeAudit) {
	audit := &fakeAudit{}
	store := &fakeStore{repos: webhook.Repos{
		Endpoint: newFakeEndpointRepo(),
		Delivery: newFakeDeliveryRepo(),
		Audit:    audit,
	}}
	u := webhook.NewUsecase(store, &fakeURLValidator{deny: deny}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return u, store, audit
}

func ownerCtx() tenant.Context {
	m := uuid.Must(uuid.NewV7())
	return tenant.Context{
		OrganizationID: uuid.Must(uuid.NewV7()),
		MembershipID:   &m,
		Role:           tenant.RoleOwner,
		PrincipalType:  tenant.PrincipalUser,
	}
}

func TestUnit_Create_Success(t *testing.T) {
	u, _, audit := testUsecase(false)
	e, secret, err := u.Create(context.Background(), ownerCtx(), webhook.CreateInput{
		URL:    "https://example.com/hook",
		Events: []string{"lead.created"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if secret == "" || e.SecretHash == "" || e.SecretHash == secret {
		t.Errorf("secret handling wrong: secret=%q hash=%q", secret, e.SecretHash)
	}
	if len(audit.actions) != 1 || audit.actions[0] != "webhook_endpoint.created" {
		t.Errorf("audit = %v, want [webhook_endpoint.created]", audit.actions)
	}
}

func TestUnit_Create_RejectsUnknownEvent(t *testing.T) {
	u, _, _ := testUsecase(false)
	_, _, err := u.Create(context.Background(), ownerCtx(), webhook.CreateInput{
		URL:    "https://example.com/hook",
		Events: []string{"lead.exploded"},
	})
	assertValidationErr(t, err, "events")
}

func TestUnit_Create_RejectsEmptyEvents(t *testing.T) {
	u, _, _ := testUsecase(false)
	_, _, err := u.Create(context.Background(), ownerCtx(), webhook.CreateInput{
		URL: "https://example.com/hook", Events: nil,
	})
	assertValidationErr(t, err, "events")
}

func TestUnit_Create_RejectsDeniedURL(t *testing.T) {
	u, _, _ := testUsecase(true) // validator denies everything
	_, _, err := u.Create(context.Background(), ownerCtx(), webhook.CreateInput{
		URL:    "http://169.254.169.254/",
		Events: []string{"lead.created"},
	})
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "webhook_url_not_allowed" || derr.Status != 400 {
		t.Errorf("expected 400 webhook_url_not_allowed, got %v", err)
	}
}

func TestUnit_Create_DeniesNonOwnerAdmin(t *testing.T) {
	u, _, _ := testUsecase(false)
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		ctx := ownerCtx()
		ctx.Role = role
		_, _, err := u.Create(context.Background(), ctx, webhook.CreateInput{
			URL: "https://example.com/hook", Events: []string{"lead.created"},
		})
		var derr *httpx.DomainError
		if !errors.As(err, &derr) || derr.Code != "forbidden" {
			t.Errorf("%s: expected forbidden, got %v", role, err)
		}
	}
}

func TestUnit_Update_RevalidatesURL(t *testing.T) {
	u, _, _ := testUsecase(false)
	ctx := ownerCtx()
	e, _, err := u.Create(context.Background(), ctx, webhook.CreateInput{
		URL: "https://example.com/hook", Events: []string{"lead.created"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Flip the validator to deny, then try to point the URL somewhere new.
	u2 := webhook.NewUsecase(&fakeStore{repos: webhook.Repos{
		Endpoint: fakeRepoWith(e), Delivery: newFakeDeliveryRepo(), Audit: &fakeAudit{},
	}}, &fakeURLValidator{deny: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	newURL := "http://10.0.0.1/hook"
	_, err = u2.Update(context.Background(), ctx, e.ID, webhook.UpdateInput{URL: &newURL})
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "webhook_url_not_allowed" {
		t.Errorf("expected Update to re-validate the URL, got %v", err)
	}
}

func TestUnit_RetryDelivery_RejectsNonFailed(t *testing.T) {
	audit := &fakeAudit{}
	dr := newFakeDeliveryRepo()
	id := uuid.Must(uuid.NewV7())
	ctx := ownerCtx()
	dr.byID[id] = &webhook.Delivery{ID: id, OrganizationID: ctx.OrganizationID, Status: webhook.StatusSucceeded}
	u := webhook.NewUsecase(&fakeStore{repos: webhook.Repos{
		Endpoint: newFakeEndpointRepo(), Delivery: dr, Audit: audit,
	}}, &fakeURLValidator{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := u.RetryDelivery(context.Background(), ctx, id)
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "delivery_not_retryable" || derr.Status != 409 {
		t.Errorf("expected 409 delivery_not_retryable, got %v", err)
	}
}

func TestUnit_RetryDelivery_ResetsFailed(t *testing.T) {
	dr := newFakeDeliveryRepo()
	id := uuid.Must(uuid.NewV7())
	ctx := ownerCtx()
	dr.byID[id] = &webhook.Delivery{ID: id, OrganizationID: ctx.OrganizationID, Status: webhook.StatusFailed, Attempt: 5}
	u := webhook.NewUsecase(&fakeStore{repos: webhook.Repos{
		Endpoint: newFakeEndpointRepo(), Delivery: dr, Audit: &fakeAudit{},
	}}, &fakeURLValidator{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	d, err := u.RetryDelivery(context.Background(), ctx, id)
	if err != nil {
		t.Fatalf("RetryDelivery: %v", err)
	}
	if d.Status != webhook.StatusPending || d.Attempt != 0 {
		t.Errorf("delivery not reset: status=%s attempt=%d", d.Status, d.Attempt)
	}
}

// --- helpers ---

func fakeRepoWith(e *webhook.Endpoint) webhook.Repository {
	r := newFakeEndpointRepo()
	r.byID[e.ID] = e
	return r
}

func assertValidationErr(t *testing.T, err error, field string) {
	t.Helper()
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	for _, d := range verr.Details {
		if d.Field == field {
			return
		}
	}
	t.Errorf("expected a detail for field %q, got %+v", field, verr.Details)
}
