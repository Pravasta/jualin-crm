package invitation_test

// Integration tests (real Postgres via dbtest) — proves the unique
// "one pending invitation per (org, email)" constraint, the atomic
// accept-new-user transaction, and the full accept-existing-user flow
// against real tables, none of which the fakes in usecase_unit_test.go
// can prove on their own.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/invitation"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/mailer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) invitation.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(invitation.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(testRepos(tx)) })
}

func (s *testStore) Repos() invitation.Repos { return testRepos(s.pool) }

func testRepos(q db.Querier) invitation.Repos {
	return invitation.Repos{
		User:       user.New(q),
		Member:     membership.New(q),
		Org:        organization.New(q),
		Audit:      auditlog.New(q),
		Invitation: invitation.New(q),
	}
}

// linkSpyMailer records every message it's asked to send — used to
// recover the raw invitation token embedded in the link, the same way
// #9/#10's auth tests do (the raw token is one-way hashed in the
// database; the mailed link is the only place a test can observe it
// after the fact). Named distinctly from usecase_unit_test.go's
// spyMailer (same package) which only counts sends.
type linkSpyMailer struct {
	sent    atomic.Int32
	lastURL atomic.Value
}

func (m *linkSpyMailer) Send(_ context.Context, msg mailer.Message) error {
	m.sent.Add(1)
	m.lastURL.Store(msg.Body)
	return nil
}

func (m *linkSpyMailer) lastToken(t *testing.T) string {
	t.Helper()
	body, _ := m.lastURL.Load().(string)
	const marker = "?token="
	i := strings.Index(body, marker)
	if i == -1 {
		t.Fatalf("no invitation link found in last sent email body: %q", body)
	}
	rest := body[i+len(marker):]
	end := strings.IndexAny(rest, " \n")
	if end == -1 {
		end = len(rest)
	}
	return rest[:end]
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func seedOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, $2)`, id, name); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return id
}

func seedOwnerMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Owner')`, userID, email); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	membershipID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'owner')`, membershipID, orgID, userID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return membershipID
}

func TestUsecase_Create_DuplicatePending_ReturnsConflict(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	u := invitation.NewUsecase(newTestStore(pool), &linkSpyMailer{}, testLogger(), "http://localhost:3000", openSeatQuota())

	orgID := seedOrg(t, ctx, pool, "Toko Dup")
	ownerMembershipID := seedOwnerMembership(t, ctx, pool, orgID, "dup-owner@example.com")
	actor := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembershipID, Role: tenant.RoleOwner}

	in := invitation.CreateInput{Email: "invitee@example.com", Role: tenant.RoleEmployee}
	if _, err := u.Create(ctx, actor, in); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := u.Create(ctx, actor, in)
	if err == nil {
		t.Fatal("expected an error for a duplicate pending invitation, got nil")
	}
}

func TestUsecase_AcceptNewUser_CreatesUserMembershipAndMarksAccepted(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &linkSpyMailer{}
	u := invitation.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", openSeatQuota())

	orgID := seedOrg(t, ctx, pool, "Toko Accept")
	ownerMembershipID := seedOwnerMembership(t, ctx, pool, orgID, "accept-owner@example.com")
	actor := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembershipID, Role: tenant.RoleOwner}

	inv, err := u.Create(ctx, actor, invitation.CreateInput{Email: "newperson@example.com", Role: tenant.RoleEmployee})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	rawToken := m.lastToken(t)

	out, err := u.Accept(ctx, nil, rawToken, "New Person", "a password long enough")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	var role string
	if err := pool.QueryRow(ctx, `SELECT role FROM memberships WHERE id = $1`, out.MembershipID).Scan(&role); err != nil {
		t.Fatalf("query membership: %v", err)
	}
	if role != string(tenant.RoleEmployee) {
		t.Errorf("expected employee role, got %q", role)
	}

	var acceptedAt *string
	if err := pool.QueryRow(ctx, `SELECT accepted_at::text FROM invitations WHERE id = $1`, inv.ID).Scan(&acceptedAt); err != nil {
		t.Fatalf("query invitation: %v", err)
	}
	if acceptedAt == nil {
		t.Error("expected accepted_at to be set")
	}

	// B3: accepting an invitation also verifies the email — caught as a
	// real bug during #11's manual smoke test (login failed with
	// email_not_verified right after a fresh accept) before this
	// assertion existed.
	var emailVerifiedAt *string
	if err := pool.QueryRow(ctx, `SELECT email_verified_at::text FROM users WHERE id = $1`, out.UserID).Scan(&emailVerifiedAt); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if emailVerifiedAt == nil {
		t.Error("expected email_verified_at to be set on invitation accept (B3)")
	}
}

func TestUsecase_AcceptExistingUser_RequiresMatchingAuthenticatedUser(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &linkSpyMailer{}
	u := invitation.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", openSeatQuota())

	orgID := seedOrg(t, ctx, pool, "Toko Existing")
	ownerMembershipID := seedOwnerMembership(t, ctx, pool, orgID, "existing-owner@example.com")
	actor := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembershipID, Role: tenant.RoleOwner}

	existingUserID := uuid.Must(uuid.NewV7())
	originalHash := "original-hash-untouched"
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, $3, 'Existing')`,
		existingUserID, "existing-invitee@example.com", originalHash); err != nil {
		t.Fatalf("seed existing user: %v", err)
	}

	if _, err := u.Create(ctx, actor, invitation.CreateInput{Email: "existing-invitee@example.com", Role: tenant.RoleManager}); err != nil {
		t.Fatalf("create: %v", err)
	}
	rawToken := m.lastToken(t)

	// Unauthenticated → rejected, password untouched.
	if _, err := u.Accept(ctx, nil, rawToken, "", ""); err == nil {
		t.Fatal("expected an error for an unauthenticated accept of an existing-user invitation")
	}
	var hashAfterRejection string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, existingUserID).Scan(&hashAfterRejection); err != nil {
		t.Fatalf("query password hash: %v", err)
	}
	if hashAfterRejection != originalHash {
		t.Error("password_hash must not change on a rejected unauthenticated accept")
	}

	// Authenticated as the correct user → succeeds.
	authenticated := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, UserID: &existingUserID}
	out, err := u.Accept(ctx, &authenticated, rawToken, "", "")
	if err != nil {
		t.Fatalf("accept as correct user: %v", err)
	}
	if out.UserID != existingUserID {
		t.Errorf("expected user_id %s, got %s", existingUserID, out.UserID)
	}

	var role string
	if err := pool.QueryRow(ctx, `SELECT role FROM memberships WHERE id = $1`, out.MembershipID).Scan(&role); err != nil {
		t.Fatalf("query membership: %v", err)
	}
	if role != string(tenant.RoleManager) {
		t.Errorf("expected manager role, got %q", role)
	}
}

func TestUsecase_Revoke_MarksRevokedAndPreventsAccept(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &linkSpyMailer{}
	u := invitation.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", openSeatQuota())

	orgID := seedOrg(t, ctx, pool, "Toko Revoke")
	ownerMembershipID := seedOwnerMembership(t, ctx, pool, orgID, "revoke-owner@example.com")
	actor := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembershipID, Role: tenant.RoleOwner}

	inv, err := u.Create(ctx, actor, invitation.CreateInput{Email: "revoked@example.com", Role: tenant.RoleEmployee})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	rawToken := m.lastToken(t)

	if err := u.Revoke(ctx, actor, inv.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	_, err = u.Accept(ctx, nil, rawToken, "Someone", "a password long enough")
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_token" {
		t.Fatalf("expected invalid_token for a revoked invitation, got: %v", err)
	}
}
