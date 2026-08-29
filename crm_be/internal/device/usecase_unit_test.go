package device_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/device/... -run TestUnit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/device"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/push"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeDeviceRepo struct {
	byToken map[string]*device.Token
	deleted []string
}

func newFakeDeviceRepo() *fakeDeviceRepo {
	return &fakeDeviceRepo{byToken: map[string]*device.Token{}}
}

func (f *fakeDeviceRepo) Upsert(_ context.Context, t tenant.Context, tok *device.Token) error {
	tok.OrganizationID = t.OrganizationID
	f.byToken[tok.Token] = tok
	return nil
}

func (f *fakeDeviceRepo) FindByToken(_ context.Context, t tenant.Context, token string) (*device.Token, error) {
	tok, ok := f.byToken[token]
	if !ok || tok.OrganizationID != t.OrganizationID {
		return nil, httpx.ErrNotFound
	}
	return tok, nil
}

func (f *fakeDeviceRepo) FindByMembership(_ context.Context, t tenant.Context, membershipID uuid.UUID) ([]*device.Token, error) {
	var out []*device.Token
	for _, tok := range f.byToken {
		if tok.OrganizationID == t.OrganizationID && tok.MembershipID == membershipID {
			out = append(out, tok)
		}
	}
	return out, nil
}

func (f *fakeDeviceRepo) DeleteByToken(_ context.Context, t tenant.Context, token string) error {
	tok, ok := f.byToken[token]
	if !ok || tok.OrganizationID != t.OrganizationID {
		return nil // matches postgresRepository.DeleteByToken: not-found is not an error
	}
	delete(f.byToken, token)
	f.deleted = append(f.deleted, token)
	return nil
}

type fakeStore struct {
	repo *fakeDeviceRepo
}

func newFakeStore() *fakeStore {
	return &fakeStore{repo: newFakeDeviceRepo()}
}

func (s *fakeStore) InTx(_ context.Context, fn func(device.Repos) error) error {
	return fn(device.Repos{DeviceToken: s.repo})
}
func (s *fakeStore) Repos() device.Repos {
	return device.Repos{DeviceToken: s.repo}
}

// fakeSender records every Send call and returns whatever this test
// queued up for that token — never touches a network.
type fakeSender struct {
	results map[string]error // token -> error to return (nil key never set = success)
	calls   []push.Message
}

func newFakeSender() *fakeSender {
	return &fakeSender{results: map[string]error{}}
}

func (f *fakeSender) Send(_ context.Context, msg push.Message) error {
	f.calls = append(f.calls, msg)
	return f.results[msg.Token]
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func actorContext(orgID, membershipID uuid.UUID, role tenant.Role) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: role}
}

// --- Register ---

func TestUnit_Register_EveryRoleAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin, tenant.RoleManager, tenant.RoleEmployee} {
		t.Run(string(role), func(t *testing.T) {
			u := device.NewUsecase(newFakeStore(), newFakeSender(), testLogger())
			org := uuid.Must(uuid.NewV7())
			tc := actorContext(org, uuid.Must(uuid.NewV7()), role)

			tok, err := u.Register(context.Background(), tc, device.RegisterInput{Token: "tok-1", Platform: "android"})
			if err != nil {
				t.Fatalf("expected role %s to be allowed to register, got error: %v", role, err)
			}
			if tok.Platform != "android" {
				t.Errorf("expected platform android, got %s", tok.Platform)
			}
		})
	}
}

func TestUnit_Register_EmptyToken_Returns400(t *testing.T) {
	u := device.NewUsecase(newFakeStore(), newFakeSender(), testLogger())
	tc := actorContext(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	_, err := u.Register(context.Background(), tc, device.RegisterInput{Token: "", Platform: "android"})
	var ve *httpx.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected a validation error for empty token, got %v", err)
	}
}

func TestUnit_Register_InvalidPlatform_Returns400(t *testing.T) {
	u := device.NewUsecase(newFakeStore(), newFakeSender(), testLogger())
	tc := actorContext(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	_, err := u.Register(context.Background(), tc, device.RegisterInput{Token: "tok-1", Platform: "windows-phone"})
	var ve *httpx.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected a validation error for an invalid platform, got %v", err)
	}
}

// --- Unregister ---

func TestUnit_Unregister_OwnToken_Succeeds(t *testing.T) {
	store := newFakeStore()
	u := device.NewUsecase(store, newFakeSender(), testLogger())
	org := uuid.Must(uuid.NewV7())
	membershipID := uuid.Must(uuid.NewV7())
	tc := actorContext(org, membershipID, tenant.RoleEmployee)

	if _, err := u.Register(context.Background(), tc, device.RegisterInput{Token: "own-token", Platform: "android"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := u.Unregister(context.Background(), tc, "own-token"); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	if len(store.repo.deleted) != 1 || store.repo.deleted[0] != "own-token" {
		t.Errorf("expected own-token to be deleted, got deletions: %v", store.repo.deleted)
	}
}

// TestUnit_Unregister_SomeoneElsesToken_Returns404NotForbidden is the
// direct behavior device.Usecase.Unregister's doc comment describes:
// a token that exists in this organization but belongs to a DIFFERENT
// membership is treated exactly like one that doesn't exist — 404, not
// 403 — so a client can't learn who else has a device registered
// (Rule #6's reasoning, extended from cross-org to cross-membership
// ownership within the same org).
func TestUnit_Unregister_SomeoneElsesToken_Returns404NotForbidden(t *testing.T) {
	store := newFakeStore()
	u := device.NewUsecase(store, newFakeSender(), testLogger())
	org := uuid.Must(uuid.NewV7())
	victim := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	attacker := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	if _, err := u.Register(context.Background(), victim, device.RegisterInput{Token: "victim-token", Platform: "android"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	err := u.Unregister(context.Background(), attacker, "victim-token")
	if err != httpx.ErrNotFound {
		t.Errorf("expected httpx.ErrNotFound, got %v", err)
	}
	if len(store.repo.deleted) != 0 {
		t.Errorf("expected victim's token to survive an unregister attempt by someone else, got deletions: %v", store.repo.deleted)
	}
}

func TestUnit_Unregister_UnknownToken_Returns404(t *testing.T) {
	u := device.NewUsecase(newFakeStore(), newFakeSender(), testLogger())
	tc := actorContext(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	err := u.Unregister(context.Background(), tc, "never-registered")
	if err != httpx.ErrNotFound {
		t.Errorf("expected httpx.ErrNotFound, got %v", err)
	}
}

// --- PushToMembership ---

func TestUnit_PushToMembership_SendsToEveryRegisteredDevice(t *testing.T) {
	store := newFakeStore()
	sender := newFakeSender()
	u := device.NewUsecase(store, sender, testLogger())
	org := uuid.Must(uuid.NewV7())
	recipient := uuid.Must(uuid.NewV7())

	regCtx := actorContext(org, recipient, tenant.RoleEmployee)
	if _, err := u.Register(context.Background(), regCtx, device.RegisterInput{Token: "device-1", Platform: "android"}); err != nil {
		t.Fatalf("register device-1: %v", err)
	}
	if _, err := u.Register(context.Background(), regCtx, device.RegisterInput{Token: "device-2", Platform: "android"}); err != nil {
		t.Fatalf("register device-2: %v", err)
	}

	actor := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner) // a DIFFERENT person doing the assigning
	if err := u.PushToMembership(context.Background(), actor, recipient, "title", "body", nil); err != nil {
		t.Fatalf("PushToMembership: %v", err)
	}

	if len(sender.calls) != 2 {
		t.Fatalf("expected 2 push sends (one per device), got %d", len(sender.calls))
	}
}

// TestUnit_PushToMembership_UnregisteredToken_IsDeleted is Phase 5
// TD §9.4, kriteria #12's direct proof: a token FCM says will never
// work again must actually be removed.
func TestUnit_PushToMembership_UnregisteredToken_IsDeleted(t *testing.T) {
	store := newFakeStore()
	sender := newFakeSender()
	sender.results["dead-token"] = push.ErrTokenInvalid
	u := device.NewUsecase(store, sender, testLogger())
	org := uuid.Must(uuid.NewV7())
	recipient := uuid.Must(uuid.NewV7())

	regCtx := actorContext(org, recipient, tenant.RoleEmployee)
	if _, err := u.Register(context.Background(), regCtx, device.RegisterInput{Token: "dead-token", Platform: "android"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	actor := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	if err := u.PushToMembership(context.Background(), actor, recipient, "t", "b", nil); err != nil {
		t.Fatalf("PushToMembership: %v", err)
	}

	if len(store.repo.deleted) != 1 || store.repo.deleted[0] != "dead-token" {
		t.Errorf("expected dead-token to be deleted after an UNREGISTERED-class send failure, got deletions: %v", store.repo.deleted)
	}
}

// TestUnit_PushToMembership_TransientError_TokenIsKept proves the
// opposite of the test above: a transient send failure must NOT delete
// anything, or a momentary FCM/network hiccup would silently and
// permanently disable a real user's push.
func TestUnit_PushToMembership_TransientError_TokenIsKept(t *testing.T) {
	store := newFakeStore()
	sender := newFakeSender()
	sender.results["flaky-token"] = errors.New("push: fcm returned 500: internal error")
	u := device.NewUsecase(store, sender, testLogger())
	org := uuid.Must(uuid.NewV7())
	recipient := uuid.Must(uuid.NewV7())

	regCtx := actorContext(org, recipient, tenant.RoleEmployee)
	if _, err := u.Register(context.Background(), regCtx, device.RegisterInput{Token: "flaky-token", Platform: "android"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	actor := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	if err := u.PushToMembership(context.Background(), actor, recipient, "t", "b", nil); err != nil {
		t.Fatalf("PushToMembership: %v", err)
	}

	if len(store.repo.deleted) != 0 {
		t.Errorf("expected flaky-token to survive a transient send error, got deletions: %v", store.repo.deleted)
	}
}

// TestUnit_PushToMembership_NoDevicesRegistered_DoesNotError covers the
// common case: an employee who has never opened the mobile app. Freeze
// A3 — the notification row is already the source of truth by the time
// this runs; a recipient with zero devices must not look like a
// failure at this call site.
func TestUnit_PushToMembership_NoDevicesRegistered_DoesNotError(t *testing.T) {
	u := device.NewUsecase(newFakeStore(), newFakeSender(), testLogger())
	org := uuid.Must(uuid.NewV7())
	actor := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	if err := u.PushToMembership(context.Background(), actor, uuid.Must(uuid.NewV7()), "t", "b", nil); err != nil {
		t.Errorf("expected no error when the recipient has no registered devices, got %v", err)
	}
}
