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
	"time"

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

func (f *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	u, ok := f.byID[id]
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

func (f *fakeUserRepo) UpdatePassword(_ context.Context, id uuid.UUID, passwordHash string) error {
	if u, ok := f.byID[id]; ok {
		u.PasswordHash = passwordHash
	}
	return nil
}

// markVerified is a test helper — bypasses MarkEmailVerified's real-time
// requirement so Login tests can set up an already-verified user without
// going through Register+VerifyEmail first.
func (f *fakeUserRepo) markVerified(id uuid.UUID, at time.Time) {
	if u, ok := f.byID[id]; ok {
		u.EmailVerifiedAt = &at
	}
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

func (f *fakeOrgRepo) FindByID(_ context.Context, id uuid.UUID) (*organization.Organization, error) {
	o, ok := f.byID[id]
	if !ok {
		return nil, httpx.ErrNotFound
	}
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

func (f *fakeMembershipRepo) FindByID(_ context.Context, _ tenant.Context, id uuid.UUID) (*membership.Membership, error) {
	for _, m := range f.all {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, httpx.ErrNotFound
}

type fakeSubRepo struct{}

func (f *fakeSubRepo) CreateFree(_ context.Context, t tenant.Context, id uuid.UUID) (*subscription.Subscription, error) {
	return &subscription.Subscription{ID: id, OrganizationID: t.OrganizationID, PlanCode: "free", Status: "active"}, nil
}

// fakePlanRepo satisfies subscription.Repository — every organization is
// on an active free plan, same as fakeSubRepo.CreateFree above. Wrapped
// in the real subscription.NewUsecase (not hand-rolled here) so
// TestUnit_Me_ReturnsProfile exercises the real channelsFor resolution
// instead of a second, possibly-drifting copy of it.
type fakePlanRepo struct{}

// fakeCounters stands in for the three meters GET /v1/me reports
// (#122). Fixed numbers rather than a simulated store: what Me must do
// is ASK and pass through, and a fake that computes its own answer
// would make a passing test say nothing about whether it did.
type fakeCounters struct{}

func (fakeCounters) CountCreatedThisMonth(context.Context, tenant.Context) (int, error) { return 7, nil }
func (fakeCounters) CountActive(context.Context, tenant.Context) (int, error)           { return 2, nil }
func (fakeCounters) CountPendingSeats(context.Context, tenant.Context) (int, error)     { return 1, nil }

func (f *fakePlanRepo) FindActiveByOrg(_ context.Context, t tenant.Context) (*subscription.Subscription, error) {
	return &subscription.Subscription{OrganizationID: t.OrganizationID, PlanCode: subscription.PlanFree, Status: "active"}, nil
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

// fakeResetRepo mirrors fakeVerifyRepo exactly — same shape, same
// used/expired-collapse-to-not-found behavior.
type fakeResetToken struct {
	userID uuid.UUID
	used   bool
	entity *auth.PasswordResetToken
}

type fakeResetRepo struct{ byHash map[string]*fakeResetToken }

func newFakeResetRepo() *fakeResetRepo {
	return &fakeResetRepo{byHash: map[string]*fakeResetToken{}}
}

func (f *fakeResetRepo) Create(_ context.Context, id, userID uuid.UUID, tokenHash string) error {
	f.byHash[tokenHash] = &fakeResetToken{
		userID: userID,
		entity: &auth.PasswordResetToken{ID: id, UserID: userID, TokenHash: tokenHash},
	}
	return nil
}

func (f *fakeResetRepo) FindValidByHash(_ context.Context, hash string) (*auth.PasswordResetToken, error) {
	t, ok := f.byHash[hash]
	if !ok || t.used {
		return nil, httpx.ErrNotFound
	}
	return t.entity, nil
}

func (f *fakeResetRepo) MarkUsed(_ context.Context, id uuid.UUID) error {
	for _, t := range f.byHash {
		if t.entity.ID == id {
			t.used = true
		}
	}
	return nil
}

// fakeRefreshTokenRepo is an in-memory refresh_tokens table. It skips
// FindByHashForUpdate's real row-locking — that guarantee only means
// something against real Postgres and is proven in usecase_test.go
// instead; here it's purely a lookup.
type fakeRefreshTokenRepo struct{ byHash map[string]*auth.RefreshToken }

func newFakeRefreshTokenRepo() *fakeRefreshTokenRepo {
	return &fakeRefreshTokenRepo{byHash: map[string]*auth.RefreshToken{}}
}

func (f *fakeRefreshTokenRepo) Create(_ context.Context, rt *auth.RefreshToken) error {
	cp := *rt
	f.byHash[rt.TokenHash] = &cp
	return nil
}

func (f *fakeRefreshTokenRepo) FindByHashForUpdate(_ context.Context, hash string) (*auth.RefreshToken, error) {
	rt, ok := f.byHash[hash]
	if !ok {
		return nil, httpx.ErrNotFound
	}
	return rt, nil
}

func (f *fakeRefreshTokenRepo) MarkReplaced(_ context.Context, id, replacedByID uuid.UUID) error {
	for _, rt := range f.byHash {
		if rt.ID == id {
			rt.ReplacedByID = &replacedByID
		}
	}
	return nil
}

func (f *fakeRefreshTokenRepo) RevokeFamily(_ context.Context, familyID uuid.UUID) error {
	now := time.Now()
	for _, rt := range f.byHash {
		if rt.FamilyID == familyID && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeRefreshTokenRepo) RevokeByID(_ context.Context, id uuid.UUID) error {
	now := time.Now()
	for _, rt := range f.byHash {
		if rt.ID == id && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeRefreshTokenRepo) RevokeAllByUserID(_ context.Context, userID uuid.UUID) error {
	now := time.Now()
	for _, rt := range f.byHash {
		if rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	_ = userID // the fake has no membership->user join; tests scope by using one user's memberships only
	return nil
}

// fakeStore runs fn against a single shared Repos with no real
// transaction — sufficient for testing business logic branches. Real
// atomicity (rollback on failure) is what the testcontainers-backed
// tests in usecase_test.go prove against actual PostgreSQL.
type fakeStore struct{ repos auth.Repos }

func newFakeStore() *fakeStore {
	return &fakeStore{repos: auth.Repos{
		User:         newFakeUserRepo(),
		Org:          newFakeOrgRepo(),
		Member:       &fakeMembershipRepo{},
		Sub:          &fakeSubRepo{},
		Plan:         subscription.NewUsecase(&fakePlanRepo{}),
		LeadCount:    fakeCounters{},
		SeatCount:    fakeCounters{},
		PendingSeats: fakeCounters{},
		Verify:       newFakeVerifyRepo(),
		Audit:        &fakeAuditRepo{},
		RefreshToken: newFakeRefreshTokenRepo(),
		ResetToken:   newFakeResetRepo(),
	}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(auth.Repos) error) error { return fn(s.repos) }
func (s *fakeStore) Repos() auth.Repos                                       { return s.repos }

func unitLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- tests ---

func TestUnit_Register_WeakPassword_Rejected(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

	in := validInput("weak@example.com")
	in.Password = "short"

	_, err := u.Register(context.Background(), in)
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got: %v", err)
	}
}

func TestUnit_Register_InvalidEmail_Rejected(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

	in := validInput("not-an-email")
	_, err := u.Register(context.Background(), in)

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for malformed email, got: %v", err)
	}
}

func TestUnit_Register_MissingOrganizationName_Rejected(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

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
	u := auth.NewUsecase(newFakeStore(), m, unitLogger(), "http://localhost:3000", testTokenConfig())

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
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

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
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

	err := u.VerifyEmail(context.Background(), "does-not-exist")

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_token" {
		t.Fatalf("expected invalid_token DomainError, got: %v", err)
	}
}

func TestUnit_ResendVerification_UnknownEmail_SendsNothing(t *testing.T) {
	m := &spyMailer{}
	u := auth.NewUsecase(newFakeStore(), m, unitLogger(), "http://localhost:3000", testTokenConfig())

	// Must not panic — same anti-enumeration guarantee as the
	// integration test, verified here without a database.
	u.ResendVerification(context.Background(), "nobody@example.com")

	if m.sentCount.Load() != 0 {
		t.Errorf("expected no email sent for an unknown address, got %d", m.sentCount.Load())
	}
}

// registerAndVerify registers a user through the real Register path and
// marks their email verified directly on the fake — Login tests aren't
// exercising the verification flow itself (that's covered above), only
// its precondition.
func registerAndVerify(t *testing.T, u *auth.Usecase, store *fakeStore, email, password string) *auth.RegisterOutput {
	t.Helper()
	in := validInput(email)
	in.Password = password
	out, err := u.Register(context.Background(), in)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	store.repos.User.(*fakeUserRepo).markVerified(out.UserID, time.Now())
	return out
}

const loginPassword = "correct horse battery staple"

func TestUnit_Login_Success(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())
	reg := registerAndVerify(t, u, store, "login-ok@example.com", loginPassword)

	out, err := u.Login(context.Background(), auth.LoginInput{
		Email: "login-ok@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Error("expected non-empty access and refresh tokens")
	}
	if out.OrganizationID != reg.OrganizationID {
		t.Errorf("expected organization_id %s, got %s", reg.OrganizationID, out.OrganizationID)
	}
	if out.Role != tenant.RoleOwner {
		t.Errorf("expected role owner, got %q", out.Role)
	}
}

func TestUnit_Login_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())
	registerAndVerify(t, u, store, "login-wrongpw@example.com", loginPassword)

	_, err := u.Login(context.Background(), auth.LoginInput{
		Email: "login-wrongpw@example.com", Password: "totally wrong password", Client: auth.ClientDashboard,
	})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials DomainError, got: %v", err)
	}
}

func TestUnit_Login_UnknownEmail_ReturnsInvalidCredentials(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

	_, err := u.Login(context.Background(), auth.LoginInput{
		Email: "nobody-logs-in@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials DomainError, got: %v", err)
	}
}

// TestUnit_Login_UnverifiedEmail_Returns403 is a direct acceptance
// criterion (TD phase 1 checklist): login must be rejected with a code a
// client can distinguish from wrong credentials.
func TestUnit_Login_UnverifiedEmail_Returns403(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

	in := validInput("login-unverified@example.com")
	in.Password = loginPassword
	if _, err := u.Register(context.Background(), in); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := u.Login(context.Background(), auth.LoginInput{
		Email: "login-unverified@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "email_not_verified" {
		t.Fatalf("expected email_not_verified DomainError, got: %v", err)
	}
}

// TestUnit_Login_MultipleMemberships_RequiresSelection is the TD §6.2
// two-step login: a user with more than one active membership and no
// organization_id gets asked to choose, not logged into an arbitrary one.
func TestUnit_Login_MultipleMemberships_RequiresSelection(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())
	reg := registerAndVerify(t, u, store, "multi-org@example.com", loginPassword)

	secondOrgID := uuid.Must(uuid.NewV7())
	if _, err := store.repos.Org.Create(context.Background(), secondOrgID, "Second Org"); err != nil {
		t.Fatalf("create second org: %v", err)
	}
	secondMembershipID := uuid.Must(uuid.NewV7())
	secondT := tenant.Context{OrganizationID: secondOrgID, PrincipalType: tenant.PrincipalUser}
	if _, err := store.repos.Member.Create(context.Background(), secondT, secondMembershipID, reg.UserID, tenant.RoleAdmin); err != nil {
		t.Fatalf("create second membership: %v", err)
	}

	_, err := u.Login(context.Background(), auth.LoginInput{
		Email: "multi-org@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})

	var selErr *auth.OrganizationSelectionError
	if !errors.As(err, &selErr) {
		t.Fatalf("expected OrganizationSelectionError, got: %v", err)
	}
	if len(selErr.Organizations) != 2 {
		t.Errorf("expected 2 organization options, got %d", len(selErr.Organizations))
	}

	// Selecting the second organization explicitly must succeed.
	out, err := u.Login(context.Background(), auth.LoginInput{
		Email: "multi-org@example.com", Password: loginPassword, Client: auth.ClientDashboard,
		OrganizationID: &secondOrgID,
	})
	if err != nil {
		t.Fatalf("login with organization_id: %v", err)
	}
	if out.OrganizationID != secondOrgID {
		t.Errorf("expected organization_id %s, got %s", secondOrgID, out.OrganizationID)
	}

	// An organization_id that doesn't match either membership must not
	// leak which organizations exist — same error as wrong credentials.
	bogusOrgID := uuid.Must(uuid.NewV7())
	_, err = u.Login(context.Background(), auth.LoginInput{
		Email: "multi-org@example.com", Password: loginPassword, Client: auth.ClientDashboard,
		OrganizationID: &bogusOrgID,
	})
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials for a non-matching organization_id, got: %v", err)
	}
}

func TestUnit_Refresh_RotatesToken(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())
	registerAndVerify(t, u, store, "refresh-rotate@example.com", loginPassword)

	login, err := u.Login(context.Background(), auth.LoginInput{
		Email: "refresh-rotate@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	out, err := u.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if out.RefreshToken == login.RefreshToken {
		t.Error("expected a new refresh token, got the same one back")
	}
	if out.AccessToken == "" {
		t.Error("expected a non-empty access token")
	}
}

// TestUnit_Refresh_ReuseDetected_RevokesFamily is TD §4's core security
// property: replaying an already-rotated refresh token doesn't just fail
// itself — it kills the token that replaced it too, because either token
// being replayed means a copy was stolen and both are now suspect.
func TestUnit_Refresh_ReuseDetected_RevokesFamily(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())
	registerAndVerify(t, u, store, "refresh-reuse@example.com", loginPassword)

	login, err := u.Login(context.Background(), auth.LoginInput{
		Email: "refresh-reuse@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	rotated, err := u.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	// Replaying the ORIGINAL (now-rotated) token is the reuse signal.
	_, err = u.Refresh(context.Background(), login.RefreshToken)
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials on reuse, got: %v", err)
	}

	// The token issued by the rotation must ALSO be dead now — the whole
	// family was revoked, not just the replayed token.
	_, err = u.Refresh(context.Background(), rotated.RefreshToken)
	if !errors.As(err, &derr) || derr.Code != "invalid_credentials" {
		t.Fatalf("expected the rotated token's family to be revoked too, got: %v", err)
	}
}

func TestUnit_Logout_RevokesToken(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())
	registerAndVerify(t, u, store, "logout@example.com", loginPassword)

	login, err := u.Login(context.Background(), auth.LoginInput{
		Email: "logout@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := u.Logout(context.Background(), login.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if _, err := u.Refresh(context.Background(), login.RefreshToken); err == nil {
		t.Error("expected refresh to fail after logout, got nil error")
	}
}

func TestUnit_Logout_UnknownToken_ReturnsNil(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

	if err := u.Logout(context.Background(), "never-issued"); err != nil {
		t.Errorf("expected nil (idempotent) for an unknown token, got: %v", err)
	}
}

// TestUnit_ResetPassword_RevokesAllSessions is the direct acceptance
// criterion: "Reset password mencabut seluruh refresh token milik user".
func TestUnit_ResetPassword_RevokesAllSessions(t *testing.T) {
	store := newFakeStore()
	m := &spyMailer{}
	u := auth.NewUsecase(store, m, unitLogger(), "http://localhost:3000", testTokenConfig())
	registerAndVerify(t, u, store, "reset-revoke@example.com", loginPassword)

	login, err := u.Login(context.Background(), auth.LoginInput{
		Email: "reset-revoke@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	u.ForgotPassword(context.Background(), "reset-revoke@example.com")
	rawResetToken := m.lastToken(t)

	if err := u.ResetPassword(context.Background(), rawResetToken, "a brand new password 123"); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	if _, err := u.Refresh(context.Background(), login.RefreshToken); err == nil {
		t.Error("expected the pre-reset refresh token to be revoked, got nil error")
	}

	// The new password must actually work.
	_, err = u.Login(context.Background(), auth.LoginInput{
		Email: "reset-revoke@example.com", Password: "a brand new password 123", Client: auth.ClientDashboard,
	})
	if err != nil {
		t.Errorf("expected login with the new password to succeed, got: %v", err)
	}
}

func TestUnit_ResetPassword_InvalidToken_ReturnsInvalidToken(t *testing.T) {
	u := auth.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())

	err := u.ResetPassword(context.Background(), "does-not-exist", "a brand new password 123")

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_token" {
		t.Fatalf("expected invalid_token DomainError, got: %v", err)
	}
}

func TestUnit_ForgotPassword_UnknownEmail_SendsNothing(t *testing.T) {
	m := &spyMailer{}
	u := auth.NewUsecase(newFakeStore(), m, unitLogger(), "http://localhost:3000", testTokenConfig())

	u.ForgotPassword(context.Background(), "nobody-forgot@example.com")

	if m.sentCount.Load() != 0 {
		t.Errorf("expected no email sent for an unknown address, got %d", m.sentCount.Load())
	}
}

func TestUnit_Me_ReturnsProfile(t *testing.T) {
	store := newFakeStore()
	u := auth.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", testTokenConfig())
	reg := registerAndVerify(t, u, store, "me@example.com", loginPassword)

	login, err := u.Login(context.Background(), auth.LoginInput{
		Email: "me@example.com", Password: loginPassword, Client: auth.ClientDashboard,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	t2 := tenant.Context{
		OrganizationID: reg.OrganizationID,
		PrincipalType:  tenant.PrincipalUser,
		MembershipID:   &login.MembershipID,
		UserID:         &reg.UserID,
		Role:           login.Role,
	}

	out, err := u.Me(context.Background(), t2)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if out.Email != "me@example.com" {
		t.Errorf("expected email me@example.com, got %q", out.Email)
	}
	if out.OrganizationID != reg.OrganizationID {
		t.Errorf("expected organization_id %s, got %s", reg.OrganizationID, out.OrganizationID)
	}
	if out.Role != tenant.RoleOwner {
		t.Errorf("expected role owner, got %q", out.Role)
	}
}
