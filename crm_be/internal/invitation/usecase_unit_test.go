package invitation_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/invitation/... -run TestUnit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/invitation"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/mailer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/token"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

// --- fakes ---

type fakeUser struct {
	id            uuid.UUID
	email         string
	passwordHash  string
	emailVerified bool
}

type fakeUserRepo struct{ byEmail map[string]*fakeUser }

func newFakeUserRepo() *fakeUserRepo { return &fakeUserRepo{byEmail: map[string]*fakeUser{}} }

func (f *fakeUserRepo) Create(_ context.Context, id uuid.UUID, email, passwordHash, _ string) (*user.User, error) {
	u := &fakeUser{id: id, email: email, passwordHash: passwordHash}
	f.byEmail[email] = u
	return &user.User{ID: id, Email: email, PasswordHash: passwordHash}, nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return nil, httpx.ErrNotFound
	}
	return &user.User{ID: u.id, Email: u.email, PasswordHash: u.passwordHash}, nil
}

func (f *fakeUserRepo) MarkEmailVerified(_ context.Context, id uuid.UUID) error {
	for _, u := range f.byEmail {
		if u.id == id {
			u.emailVerified = true
		}
	}
	return nil
}

type fakeMembershipRepo struct{ created []*membership.Membership }

func (f *fakeMembershipRepo) Create(_ context.Context, t tenant.Context, id, userID uuid.UUID, role tenant.Role) (*membership.Membership, error) {
	m := &membership.Membership{ID: id, OrganizationID: t.OrganizationID, UserID: userID, Role: role}
	f.created = append(f.created, m)
	return m, nil
}

// CountActive counts what this fake actually holds — half the seat
// meter (Phase 8.5 #124). Deliberately does NOT count members created
// by OTHER tests sharing a fakeMembershipRepo instance, since each test
// constructs its own.
func (f *fakeMembershipRepo) CountActive(_ context.Context, t tenant.Context) (int, error) {
	n := 0
	for _, m := range f.created {
		if m.OrganizationID == t.OrganizationID {
			n++
		}
	}
	return n, nil
}

// fakeSeatQuota lets tests control the seat gate (#124) independently
// of authz — allow (nil) by default so every pre-existing test keeps
// exercising Create's other branches unaffected. lastUsed records what
// Usecase.Create actually summed (active + pending), so a test can
// prove pending invitations were counted without needing a real quota
// rejection to observe it.
type fakeSeatQuota struct {
	reject   bool
	lastUsed int
}

func openSeatQuota() *fakeSeatQuota   { return &fakeSeatQuota{} }
func closedSeatQuota() *fakeSeatQuota { return &fakeSeatQuota{reject: true} }

func (f *fakeSeatQuota) AllowSeat(_ context.Context, _ tenant.Context, used int) error {
	f.lastUsed = used
	if f.reject {
		return &httpx.DomainError{Status: http.StatusForbidden, Code: "plan_seat_limit_reached", Message: "Paket Anda dibatasi 2 anggota. Sudah tercapai batasnya."}
	}
	return nil
}

type fakeOrgRepo struct {
	byID map[uuid.UUID]*organization.Organization
}

func newFakeOrgRepo() *fakeOrgRepo {
	return &fakeOrgRepo{byID: map[uuid.UUID]*organization.Organization{}}
}

func (f *fakeOrgRepo) FindByID(_ context.Context, id uuid.UUID) (*organization.Organization, error) {
	o, ok := f.byID[id]
	if !ok {
		return nil, httpx.ErrNotFound
	}
	return o, nil
}

type fakeAuditRepo struct{ actions []string }

func (f *fakeAuditRepo) Record(_ context.Context, _ tenant.Context, _ *uuid.UUID, action string) error {
	f.actions = append(f.actions, action)
	return nil
}

type fakeInvitationRepo struct {
	byHash map[string]*invitation.Invitation
}

func newFakeInvitationRepo() *fakeInvitationRepo {
	return &fakeInvitationRepo{byHash: map[string]*invitation.Invitation{}}
}

func (f *fakeInvitationRepo) Create(_ context.Context, inv *invitation.Invitation) error {
	cp := *inv
	f.byHash[inv.TokenHash] = &cp
	return nil
}

func (f *fakeInvitationRepo) FindByOrgPending(_ context.Context, t tenant.Context) ([]*invitation.Invitation, error) {
	var out []*invitation.Invitation
	for _, inv := range f.byHash {
		if inv.OrganizationID == t.OrganizationID && inv.AcceptedAt == nil && inv.RevokedAt == nil {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (f *fakeInvitationRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*invitation.Invitation, error) {
	for _, inv := range f.byHash {
		if inv.ID == id && inv.OrganizationID == t.OrganizationID {
			return inv, nil
		}
	}
	return nil, httpx.ErrNotFound
}

func (f *fakeInvitationRepo) FindValidByHash(_ context.Context, hash string) (*invitation.Invitation, error) {
	inv, ok := f.byHash[hash]
	if !ok {
		return nil, httpx.ErrNotFound
	}
	return inv, nil
}

func (f *fakeInvitationRepo) MarkAccepted(_ context.Context, id uuid.UUID) error {
	for _, inv := range f.byHash {
		if inv.ID == id {
			now := time.Now()
			inv.AcceptedAt = &now
		}
	}
	return nil
}

func (f *fakeInvitationRepo) MarkRevoked(_ context.Context, id uuid.UUID) error {
	for _, inv := range f.byHash {
		if inv.ID == id {
			now := time.Now()
			inv.RevokedAt = &now
		}
	}
	return nil
}

// CountPendingSeats mirrors the real predicate: still acceptable means
// not accepted, not revoked, AND not expired (#122) — exercised by
// TestUnit_Create_SeatUsed_SumsActiveAndPending below (#124).
func (f *fakeInvitationRepo) CountPendingSeats(_ context.Context, t tenant.Context) (int, error) {
	n := 0
	for _, inv := range f.byHash {
		if inv.OrganizationID == t.OrganizationID &&
			inv.AcceptedAt == nil && inv.RevokedAt == nil && inv.ExpiresAt.After(time.Now()) {
			n++
		}
	}
	return n, nil
}

type fakeStore struct{ repos invitation.Repos }

func newFakeStore() *fakeStore {
	return &fakeStore{repos: invitation.Repos{
		User:       newFakeUserRepo(),
		Member:     &fakeMembershipRepo{},
		Org:        newFakeOrgRepo(),
		Audit:      &fakeAuditRepo{},
		Invitation: newFakeInvitationRepo(),
	}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(invitation.Repos) error) error {
	return fn(s.repos)
}
func (s *fakeStore) Repos() invitation.Repos { return s.repos }

type spyMailer struct{ sent int }

func (m *spyMailer) Send(_ context.Context, _ mailer.Message) error {
	m.sent++
	return nil
}

func unitLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func actorContext(orgID, membershipID uuid.UUID, role tenant.Role) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: role}
}

// --- tests ---

func TestUnit_Create_EmployeeForbidden(t *testing.T) {
	u := invitation.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())

	_, err := u.Create(context.Background(), actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee),
		invitation.CreateInput{Email: "invited@example.com", Role: tenant.RoleEmployee})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden, got: %v", err)
	}
}

func TestUnit_Create_CannotInviteOwner(t *testing.T) {
	u := invitation.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())

	_, err := u.Create(context.Background(), actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner),
		invitation.CreateInput{Email: "invited@example.com", Role: tenant.RoleOwner})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for role=owner, got: %v", err)
	}
}

func TestUnit_Create_Success_SendsEmail(t *testing.T) {
	m := &spyMailer{}
	u := invitation.NewUsecase(newFakeStore(), m, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())

	inv, err := u.Create(context.Background(), actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleAdmin),
		invitation.CreateInput{Email: "invited@example.com", Role: tenant.RoleEmployee})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if inv.Email != "invited@example.com" {
		t.Errorf("expected email invited@example.com, got %q", inv.Email)
	}
	if m.sent != 1 {
		t.Errorf("expected 1 email sent, got %d", m.sent)
	}
}

func TestUnit_Accept_UnknownToken_ReturnsInvalidToken(t *testing.T) {
	u := invitation.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())

	_, err := u.Accept(context.Background(), nil, "does-not-exist", "Full Name", "a password long enough")

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_token" {
		t.Fatalf("expected invalid_token, got: %v", err)
	}
}

func TestUnit_Accept_NewUser_Success(t *testing.T) {
	store := newFakeStore()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	store.repos.Org.(*fakeOrgRepo).byID[orgID] = &organization.Organization{ID: orgID, Name: "Toko ABC"}

	rawToken := seedInvitation(t, store, orgID, "newbie@example.com", tenant.RoleEmployee)

	out, err := u.Accept(context.Background(), nil, rawToken, "New Person", "a password long enough")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if out.OrganizationID != orgID {
		t.Errorf("expected organization_id %s, got %s", orgID, out.OrganizationID)
	}

	members := store.repos.Member.(*fakeMembershipRepo).created
	if len(members) != 1 || members[0].Role != tenant.RoleEmployee {
		t.Fatalf("expected 1 employee membership created, got %+v", members)
	}

	// B3: accepting an invitation also verifies the email — no separate
	// verification step for an invited employee.
	created := store.repos.User.(*fakeUserRepo).byEmail["newbie@example.com"]
	if !created.emailVerified {
		t.Error("expected the new user's email to be verified on invitation accept (B3)")
	}
}

func TestUnit_Accept_NewUser_WeakPassword_Rejected(t *testing.T) {
	store := newFakeStore()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	rawToken := seedInvitation(t, store, orgID, "newbie2@example.com", tenant.RoleEmployee)

	_, err := u.Accept(context.Background(), nil, rawToken, "New Person", "short")

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for a weak password, got: %v", err)
	}
}

// TestUnit_Accept_ExistingUser_Unauthenticated_Returns401 is the direct
// acceptance criterion: "Undangan ke email yang sudah punya akun, tanpa
// autentikasi → 401, password tidak tersentuh."
func TestUnit_Accept_ExistingUser_Unauthenticated_Returns401(t *testing.T) {
	store := newFakeStore()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())

	existing, _ := store.repos.User.(*fakeUserRepo).Create(context.Background(), uuid.Must(uuid.NewV7()), "existing@example.com", "original-hash", "Existing User")
	rawToken := seedInvitation(t, store, orgID, "existing@example.com", tenant.RoleEmployee)

	_, err := u.Accept(context.Background(), nil, rawToken, "", "a completely different password")

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "authentication_required" {
		t.Fatalf("expected authentication_required, got: %v", err)
	}

	reloaded, _ := store.repos.User.(*fakeUserRepo).FindByEmail(context.Background(), "existing@example.com")
	if reloaded.PasswordHash != existing.PasswordHash {
		t.Error("password hash must not change on a rejected unauthenticated accept attempt")
	}
}

// TestUnit_Accept_ExistingUser_AuthenticatedAsDifferentUser_Rejected is
// the second acceptance criterion: authenticated as a different user
// must also be rejected, not silently succeed for the wrong account.
func TestUnit_Accept_ExistingUser_AuthenticatedAsDifferentUser_Rejected(t *testing.T) {
	store := newFakeStore()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	rawToken := seedInvitation(t, store, orgID, "existing2@example.com", tenant.RoleEmployee)
	_, _ = store.repos.User.(*fakeUserRepo).Create(context.Background(), uuid.Must(uuid.NewV7()), "existing2@example.com", "hash", "Existing User")

	differentUserID := uuid.Must(uuid.NewV7())
	authenticated := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, UserID: &differentUserID}

	_, err := u.Accept(context.Background(), &authenticated, rawToken, "", "")

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "authentication_required" {
		t.Fatalf("expected authentication_required for a different authenticated user, got: %v", err)
	}
}

func TestUnit_Accept_ExistingUser_AuthenticatedAsCorrectUser_Succeeds(t *testing.T) {
	store := newFakeStore()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	rawToken := seedInvitation(t, store, orgID, "existing3@example.com", tenant.RoleManager)
	created, _ := store.repos.User.(*fakeUserRepo).Create(context.Background(), uuid.Must(uuid.NewV7()), "existing3@example.com", "hash", "Existing User")

	authenticated := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, UserID: &created.ID}

	out, err := u.Accept(context.Background(), &authenticated, rawToken, "", "")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if out.UserID != created.ID {
		t.Errorf("expected user_id %s, got %s", created.ID, out.UserID)
	}

	members := store.repos.Member.(*fakeMembershipRepo).created
	if len(members) != 1 || members[0].Role != tenant.RoleManager {
		t.Fatalf("expected 1 manager membership created, got %+v", members)
	}
}

func TestUnit_Accept_AlreadyAccepted_ReturnsSpecificCode(t *testing.T) {
	store := newFakeStore()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	rawToken := seedInvitation(t, store, orgID, "twice@example.com", tenant.RoleEmployee)

	if _, err := u.Accept(context.Background(), nil, rawToken, "Person", "a password long enough"); err != nil {
		t.Fatalf("first accept: %v", err)
	}

	_, err := u.Accept(context.Background(), nil, rawToken, "Person", "a password long enough")
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invitation_already_accepted" {
		t.Fatalf("expected invitation_already_accepted, got: %v", err)
	}
}

func TestUnit_Revoke_CrossOrg_ReturnsNotFound(t *testing.T) {
	store := newFakeStore()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgA, orgB := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	rawTokenInOrgB := seedInvitation(t, store, orgB, "cross-org@example.com", tenant.RoleEmployee)
	_ = rawTokenInOrgB

	invID := lastCreatedInvitationID(store)

	err := u.Revoke(context.Background(), actorContext(orgA, uuid.Must(uuid.NewV7()), tenant.RoleOwner), invID)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a cross-org invitation, got: %v", err)
	}
}

func TestUnit_TokenInfo_UserExists(t *testing.T) {
	store := newFakeStore()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	store.repos.Org.(*fakeOrgRepo).byID[orgID] = &organization.Organization{ID: orgID, Name: "Toko ABC"}
	_, _ = store.repos.User.(*fakeUserRepo).Create(context.Background(), uuid.Must(uuid.NewV7()), "known@example.com", "hash", "Known User")
	rawToken := seedInvitation(t, store, orgID, "known@example.com", tenant.RoleEmployee)

	info, err := u.TokenInfo(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("token info: %v", err)
	}
	if !info.UserExists {
		t.Error("expected user_exists=true")
	}
	if info.OrganizationName != "Toko ABC" {
		t.Errorf("expected organization_name Toko ABC, got %q", info.OrganizationName)
	}
}

// seedInvitation writes an invitation directly to the fake store's
// Invitation repo and returns the raw token — mirrors Create's shape
// without going through authz/rate-limit concerns unit tests here don't
// care about.
func seedInvitation(t *testing.T, store *fakeStore, orgID uuid.UUID, email string, role tenant.Role) string {
	t.Helper()
	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	inv := &invitation.Invitation{
		ID:                    uuid.Must(uuid.NewV7()),
		OrganizationID:        orgID,
		Email:                 email,
		Role:                  role,
		TokenHash:             tokenHash,
		InvitedByMembershipID: uuid.Must(uuid.NewV7()),
		ExpiresAt:             time.Now().Add(24 * time.Hour),
	}
	if err := store.repos.Invitation.Create(context.Background(), inv); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	return rawToken
}

func lastCreatedInvitationID(store *fakeStore) uuid.UUID {
	for _, inv := range store.repos.Invitation.(*fakeInvitationRepo).byHash {
		return inv.ID
	}
	return uuid.UUID{}
}

// --- Create: seat quota (Phase 8.5 #124) ---

func TestUnit_Create_SeatOpen_Succeeds(t *testing.T) {
	u := invitation.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", openSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	actor := actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	if _, err := u.Create(context.Background(), actor, invitation.CreateInput{Email: "invited@example.com", Role: tenant.RoleEmployee}); err != nil {
		t.Fatalf("create: %v", err)
	}
}

func TestUnit_Create_SeatFull_Returns403PlanSeatLimitReached(t *testing.T) {
	u := invitation.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", closedSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	actor := actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	_, err := u.Create(context.Background(), actor, invitation.CreateInput{Email: "invited@example.com", Role: tenant.RoleEmployee})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "plan_seat_limit_reached" {
		t.Fatalf("expected plan_seat_limit_reached, got: %v", err)
	}
}

// TestUnit_Create_RoleCheckedBeforeSeatQuota is the only way to prove
// the ORDER of the two gates from outside (subscription TD §3.3,
// applied here by 8.5 TD §6): a Manager-or-lower role that cannot
// invite at all must see "forbidden", never "plan_seat_limit_reached"
// — the latter would leak the organization's billing state to someone
// with no standing to ask.
func TestUnit_Create_RoleCheckedBeforeSeatQuota(t *testing.T) {
	u := invitation.NewUsecase(newFakeStore(), &spyMailer{}, unitLogger(), "http://localhost:3000", closedSeatQuota())
	orgID := uuid.Must(uuid.NewV7())
	actor := actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleEmployee) // authz denies this role too

	_, err := u.Create(context.Background(), actor, invitation.CreateInput{Email: "invited@example.com", Role: tenant.RoleEmployee})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden (role checked first), got: %v", err)
	}
}

// TestUnit_Create_SeatUsed_SumsActiveAndPending is kriteria "undangan
// pending terbukti ikut dihitung": one active membership plus one
// pending invitation must reach the gate as used=2, not used=1 — the
// exact miscount that would let a 2-seat organization send five
// invitations and end up with five members.
func TestUnit_Create_SeatUsed_SumsActiveAndPending(t *testing.T) {
	store := newFakeStore()
	seatQuota := openSeatQuota()
	u := invitation.NewUsecase(store, &spyMailer{}, unitLogger(), "http://localhost:3000", seatQuota)
	orgID := uuid.Must(uuid.NewV7())
	actor := actorContext(orgID, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	// One active member (the owner themselves).
	if _, err := store.repos.Member.Create(context.Background(), tenant.Context{OrganizationID: orgID}, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), tenant.RoleOwner); err != nil {
		t.Fatalf("seed active member: %v", err)
	}
	// One pending invitation nobody has accepted yet.
	seedInvitation(t, store, orgID, "already-invited@example.com", tenant.RoleEmployee)

	if _, err := u.Create(context.Background(), actor, invitation.CreateInput{Email: "new@example.com", Role: tenant.RoleEmployee}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if seatQuota.lastUsed != 2 {
		t.Errorf("expected used=2 (1 active + 1 pending), got %d", seatQuota.lastUsed)
	}
}
