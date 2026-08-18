package auth_test

// TestUnit_* tests prove Usecase is genuinely decoupled from PostgreSQL
// (ADR-011) — they use a fake Store and never touch testcontainers or
// Docker. Run in isolation with:
//
//	go test ./internal/auth/... -run TestUnit
//
// which passes even with the Docker daemon stopped — that's the
// acceptance criterion this file exists to satisfy.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

// --- fakes: in-memory implementations of port.go's interfaces ---

type fakeUserRepo struct {
	byEmail map[string]*user.User
	byID    map[uuid.UUID]*user.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: map[string]*user.User{}, byID: map[uuid.UUID]*user.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, id uuid.UUID, email, passwordHash, fullName string) (*user.User, error) {
	if _, exists := f.byEmail[email]; exists {
		return nil, user.ErrEmailTaken
	}
	u := &user.User{ID: id, Email: email, PasswordHash: passwordHash, FullName: fullName}
	f.byEmail[email] = u
	f.byID[id] = u
	return u, nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return nil, httpx.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) MarkEmailVerified(_ context.Context, id uuid.UUID) error {
	if u, ok := f.byID[id]; ok {
		now := u.CreatedAt // any non-nil time works for this fake
		u.EmailVerifiedAt = &now
	}
	return nil
}

type fakeOrgRepo struct {
	byID map[uuid.UUID]*organization.Organization
}

func newFakeOrgRepo() *fakeOrgRepo {
	return &fakeOrgRepo{byID: map[uuid.UUID]*organization.Organization{}}
}

func (f *fakeOrgRepo) Create(_ context.Context, id uuid.UUID, name string) (*organization.Organization, error) {
	o := &organization.Organization{ID: id, Name: name}
	f.byID[id] = o
	return o, nil
}

type fakeMembershipRepo struct{ all []*membership.Membership }

func (f *fakeMembershipRepo) Create(_ context.Context, t tenant.Context, id, userID uuid.UUID, role tenant.Role) (*membership.Membership, error) {
	m := &membership.Membership{ID: id, OrganizationID: t.OrganizationID, UserID: userID, Role: role}
	f.all = append(f.all, m)
	return m, nil
}

func (f *fakeMembershipRepo) FindActiveByUserID(_ context.Context, userID uuid.UUID) ([]*membership.Membership, error) {
	var out []*membership.Membership
	for _, m := range f.all {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}

type fakeSubRepo struct{}

func (f *fakeSubRepo) CreateFree(_ context.Context, t tenant.Context, id uuid.UUID) (*subscription.Subscription, error) {
	return &subscription.Subscription{ID: id, OrganizationID: t.OrganizationID, PlanCode: "free", Status: "active"}, nil
}

type fakeVerifyToken struct {
	userID uuid.UUID
	used   bool
	entity *auth.EmailVerificationToken
}

type fakeVerifyRepo struct{ byHash map[string]*fakeVerifyToken }

func newFakeVerifyRepo() *fakeVerifyRepo {
	return &fakeVerifyRepo{byHash: map[string]*fakeVerifyToken{}}
}

func (f *fakeVerifyRepo) Create(_ context.Context, id, userID uuid.UUID, tokenHash string) error {
	f.byHash[tokenHash] = &fakeVerifyToken{
		userID: userID,
		entity: &auth.EmailVerificationToken{ID: id, UserID: userID, TokenHash: tokenHash},
	}
	return nil
}

func (f *fakeVerifyRepo) FindValidByHash(_ context.Context, hash string) (*auth.EmailVerificationToken, error) {
	t, ok := f.byHash[hash]
	if !ok || t.used {
		return nil, httpx.ErrNotFound
	}
	return t.entity, nil
}

func (f *fakeVerifyRepo) MarkUsed(_ context.Context, id uuid.UUID) error {
	for _, t := range f.byHash {
		if t.entity.ID == id {
			t.used = true
		}
	}
	return nil
}

type fakeAuditRepo struct{ actions []string }

func (f *fakeAuditRepo) Record(_ context.Context, _ tenant.Context, _ *uuid.UUID, action string) error {
	f.actions = append(f.actions, action)
	return nil
}

// fakeStore runs fn against a single shared Repos with no real
// transaction — sufficient for testing business logic branches. Real
// atomicity (rollback on failure) is what the testcontainers-backed
// tests in usecase_test.go prove against actual PostgreSQL.
type fakeStore struct{ repos auth.Repos }

func newFakeStore() *fakeStore {
	return &fakeStore{repos: auth.Repos{
		User:   newFakeUserRepo(),
		Org:    newFakeOrgRepo(),
		Member: &fakeMembershipRepo{},
		Sub:    &fakeSubRepo{},
		Verify: newFakeVerifyRepo(),
		Audit:  &fakeAuditRepo{},
	}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(auth.Repos) error) error { return fn(s.repos) }
func (s *fakeStore) Repos() auth.Repos                                       { return s.repos }

func unitLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- tests ---

func TestUnit_Register_WeakPassword_Rejected(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000")

	in := validInput("weak@example.com")
	in.Password = "short"

	_, err := u.Register(context.Background(), in)
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got: %v", err)
	}
}

func TestUnit_Register_InvalidEmail_Rejected(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000")

	in := validInput("not-an-email")
	_, err := u.Register(context.Background(), in)

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for malformed email, got: %v", err)
	}
}

func TestUnit_Register_MissingOrganizationName_Rejected(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000")

	in := validInput("missing-org@example.com")
	in.OrganizationName = ""
	_, err := u.Register(context.Background(), in)

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for missing organization_name, got: %v", err)
	}
}

func TestUnit_Register_Success_SendsVerificationEmail(t *testing.T) {
	m := &spyMailer{}
	u := auth.NewUsecase(newFakeStore(), m, unitLogger(), "http://localhost:3000")

	out, err := u.Register(context.Background(), validInput("unit-success@example.com"))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if out.UserID == (uuid.UUID{}) || out.OrganizationID == (uuid.UUID{}) {
		t.Error("expected non-zero UserID and OrganizationID")
	}
	if m.sentCount.Load() != 1 {
		t.Errorf("expected 1 email sent, got %d", m.sentCount.Load())
	}
}

func TestUnit_Register_DuplicateEmail_ReturnsDomainError(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000")

	if _, err := u.Register(context.Background(), validInput("unit-dup@example.com")); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	_, err := u.Register(context.Background(), validInput("unit-dup@example.com"))
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "email_already_registered" {
		t.Fatalf("expected email_already_registered DomainError, got: %v", err)
	}
}

func TestUnit_VerifyEmail_UnknownToken_ReturnsInvalidToken(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000")

	err := u.VerifyEmail(context.Background(), "does-not-exist")

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_token" {
		t.Fatalf("expected invalid_token DomainError, got: %v", err)
	}
}

func TestUnit_ResendVerification_UnknownEmail_SendsNothing(t *testing.T) {
	m := &spyMailer{}
	u := auth.NewUsecase(newFakeStore(), m, unitLogger(), "http://localhost:3000")

	// Must not panic — same anti-enumeration guarantee as the
	// integration test, verified here without a database.
	u.ResendVerification(context.Background(), "nobody@example.com")

	if m.sentCount.Load() != 0 {
		t.Errorf("expected no email sent for an unknown address, got %d", m.sentCount.Load())
	}
}
