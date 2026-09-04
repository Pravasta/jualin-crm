package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/mailer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/password"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/token"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testStore is a real, PostgreSQL-backed auth.Store — deliberately NOT a
// fake. These are integration tests proving the atomic transaction
// behavior (registration's four inserts, rollback on failure) actually
// works against real Postgres; that guarantee only means something when
// checked against the real thing. It mirrors cmd/api/auth_store.go —
// internal/auth's production code never assembles Repos itself
// (ADR-011), so test code that needs a real Store duplicates the small
// amount of wiring rather than importing the main package.
type testStore struct {
	pool *pgxpool.Pool
}

func newTestStore(pool *pgxpool.Pool) auth.Store {
	return &testStore{pool: pool}
}

func (s *testStore) InTx(ctx context.Context, fn func(auth.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(testRepos(tx))
	})
}

func (s *testStore) Repos() auth.Repos {
	return testRepos(s.pool)
}

func testRepos(q db.Querier) auth.Repos {
	return auth.Repos{
		User:         user.New(q),
		Org:          organization.New(q),
		Member:       membership.New(q),
		Sub:          subscription.New(q),
		Plan:         subscription.NewUsecase(subscription.New(q)),
		Verify:       auth.NewVerificationRepository(q),
		Audit:        auditlog.New(q),
		RefreshToken: auth.NewRefreshTokenRepository(q),
		ResetToken:   auth.NewPasswordResetRepository(q),
	}
}

// testTokenConfig is a fixed TokenConfig for tests — a real secret isn't
// needed for security, only for accesstoken.Issue/Parse to round-trip.
func testTokenConfig() auth.TokenConfig {
	return auth.TokenConfig{
		JWTSecret:                []byte("test-jwt-secret-at-least-32-bytes-long"),
		AccessTokenTTL:           15 * time.Minute,
		RefreshTokenTTLDashboard: 720 * time.Hour,
		RefreshTokenTTLMobile:    2160 * time.Hour,
	}
}

// spyMailer records every message it's asked to send, and can be told to
// fail — used to prove Rule #32: a send failure must never roll back
// work that already committed. It also captures the last message body so
// tests can pull the raw verification token out of the link
// sendVerificationEmail embeds — the raw token exists nowhere in the
// database (only its hash does), so this is the only way a test can
// recover it short of reimplementing token generation.
type spyMailer struct {
	failNext  atomic.Bool
	sentCount atomic.Int32
	lastTo    atomic.Value
	lastBody  atomic.Value
}

func (m *spyMailer) Send(_ context.Context, msg mailer.Message) error {
	if m.failNext.Load() {
		return errors.New("simulated send failure")
	}
	m.sentCount.Add(1)
	m.lastTo.Store(msg.To)
	m.lastBody.Store(msg.Body)
	return nil
}

// lastToken extracts the raw token from the "?token=<value>" query
// string embedded in the last sent message's body.
func (m *spyMailer) lastToken(t *testing.T) string {
	t.Helper()
	body, _ := m.lastBody.Load().(string)
	const marker = "?token="
	i := strings.Index(body, marker)
	if i == -1 {
		t.Fatalf("no verification link found in last sent email body: %q", body)
	}
	rest := body[i+len(marker):]
	end := strings.IndexAny(rest, " \n")
	if end == -1 {
		end = len(rest)
	}
	return rest[:end]
}

func validInput(email string) auth.RegisterInput {
	return auth.RegisterInput{
		OrganizationName: "Toko ABC",
		FullName:         "Budi Santoso",
		Email:            email,
		Password:         "correct horse battery staple",
	}
}

func TestRegister_CreatesFourRowsAtomically(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	out, err := svc.Register(ctx, validInput("owner@example.com"))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	assertRowExists(t, ctx, pool, "organizations", out.OrganizationID)
	assertRowExists(t, ctx, pool, "users", out.UserID)

	var membershipCount int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM memberships WHERE organization_id = $1 AND user_id = $2 AND role = 'owner'`,
		out.OrganizationID, out.UserID,
	).Scan(&membershipCount)
	if err != nil || membershipCount != 1 {
		t.Errorf("expected exactly 1 owner membership, got count=%d err=%v", membershipCount, err)
	}

	var subCount int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM subscriptions WHERE organization_id = $1 AND plan_code = 'free' AND status = 'active'`,
		out.OrganizationID,
	).Scan(&subCount)
	if err != nil || subCount != 1 {
		t.Errorf("expected exactly 1 free active subscription, got count=%d err=%v", subCount, err)
	}

	var auditCount int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_logs WHERE organization_id = $1 AND action = 'user.registered'`,
		out.OrganizationID,
	).Scan(&auditCount)
	if err != nil || auditCount != 1 {
		t.Errorf("expected exactly 1 user.registered audit entry, got count=%d err=%v", auditCount, err)
	}

	if m.sentCount.Load() != 1 {
		t.Errorf("expected exactly 1 email sent, got %d", m.sentCount.Load())
	}
	if got := m.lastTo.Load(); got != "owner@example.com" {
		t.Errorf("expected verification email sent to owner@example.com, got %v", got)
	}
}

// TestRegister_DuplicateEmail_LeavesNothingBehind is the acceptance
// criterion "kegagalan di tengah transaksi → tidak ada satupun baris
// tersimpan" — the second registration's organization insert must not
// survive the user insert failing.
func TestRegister_DuplicateEmail_LeavesNothingBehind(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	svc := auth.NewUsecase(newTestStore(pool), &spyMailer{}, testLogger(), "http://localhost:3000", testTokenConfig())

	if _, err := svc.Register(ctx, validInput("dup@example.com")); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	var orgCountBefore int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgCountBefore); err != nil {
		t.Fatalf("failed to count organizations: %v", err)
	}

	secondInput := validInput("dup@example.com")
	secondInput.OrganizationName = "Toko XYZ"
	_, err := svc.Register(ctx, secondInput)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "email_already_registered" {
		t.Fatalf("expected email_already_registered DomainError, got: %v", err)
	}

	var orgCountAfter int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgCountAfter); err != nil {
		t.Fatalf("failed to count organizations: %v", err)
	}
	if orgCountAfter != orgCountBefore {
		t.Errorf("expected the failed registration to leave no new organization behind: before=%d after=%d", orgCountBefore, orgCountAfter)
	}

	var userCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = 'dup@example.com'`).Scan(&userCount); err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if userCount != 1 {
		t.Errorf("expected exactly 1 user with the duplicate email (from the first, successful registration), got %d", userCount)
	}
}

// TestRegister_MailerFailure_StillCommits is Rule #32's core guarantee:
// registration is already committed by the time the email is sent, so a
// send failure cannot roll it back.
func TestRegister_MailerFailure_StillCommits(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	m.failNext.Store(true)
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	out, err := svc.Register(ctx, validInput("mailfail@example.com"))
	if err != nil {
		t.Fatalf("expected registration to succeed despite mailer failure, got: %v", err)
	}

	assertRowExists(t, ctx, pool, "users", out.UserID)
}

func TestVerifyEmail_ValidToken_MarksVerifiedAndConsumesToken(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	out, err := svc.Register(ctx, validInput("verify@example.com"))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	rawToken := m.lastToken(t)

	if err := svc.VerifyEmail(ctx, rawToken); err != nil {
		t.Fatalf("verify email failed: %v", err)
	}

	var verifiedAt *time.Time
	err = pool.QueryRow(ctx, `SELECT email_verified_at FROM users WHERE id = $1`, out.UserID).Scan(&verifiedAt)
	if err != nil || verifiedAt == nil {
		t.Fatalf("expected email_verified_at to be set, got %v (err=%v)", verifiedAt, err)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'email.verified'`).Scan(&auditCount); err != nil {
		t.Fatalf("failed to count audit logs: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected exactly 1 email.verified audit entry, got %d", auditCount)
	}
}

// TestVerifyEmail_TokenUsedTwice_SecondAttemptFails proves single-use:
// the acceptance criterion "token verifikasi sekali pakai — dipakai dua
// kali → invalid_token".
func TestVerifyEmail_TokenUsedTwice_SecondAttemptFails(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	_, err := svc.Register(ctx, validInput("reuse@example.com"))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	rawToken := m.lastToken(t)

	if err := svc.VerifyEmail(ctx, rawToken); err != nil {
		t.Fatalf("first verification failed: %v", err)
	}

	err = svc.VerifyEmail(ctx, rawToken)
	assertInvalidToken(t, err)
}

func TestVerifyEmail_ExpiredToken_Rejected(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	svc := auth.NewUsecase(newTestStore(pool), &spyMailer{}, testLogger(), "http://localhost:3000", testTokenConfig())

	out, err := svc.Register(ctx, validInput("expired@example.com"))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	rawToken, hash, err := token.Generate()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at)
		VALUES (gen_random_uuid(), $1, $2, now() - interval '1 hour')`,
		out.UserID, hash,
	)
	if err != nil {
		t.Fatalf("failed to insert expired token: %v", err)
	}

	err = svc.VerifyEmail(ctx, rawToken)
	assertInvalidToken(t, err)
}

func TestVerifyEmail_UnknownToken_Rejected(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	svc := auth.NewUsecase(newTestStore(pool), &spyMailer{}, testLogger(), "http://localhost:3000", testTokenConfig())

	err := svc.VerifyEmail(ctx, "this-token-does-not-exist")
	assertInvalidToken(t, err)
}

func TestResendVerification_UnknownEmail_SendsNothing(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	// Must not panic or error — the handler always answers 202 regardless.
	svc.ResendVerification(ctx, "nobody@example.com")

	if m.sentCount.Load() != 0 {
		t.Errorf("expected no email sent for an unknown address, got %d", m.sentCount.Load())
	}
}

func TestResendVerification_UnverifiedUser_SendsNewToken(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	out, err := svc.Register(ctx, validInput("resend@example.com"))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	svc.ResendVerification(ctx, "resend@example.com")

	if m.sentCount.Load() != 2 { // one from Register, one from resend
		t.Errorf("expected 2 emails sent total (register + resend), got %d", m.sentCount.Load())
	}

	var tokenCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM email_verification_tokens WHERE user_id = $1`, out.UserID).Scan(&tokenCount); err != nil {
		t.Fatalf("failed to count verification tokens: %v", err)
	}
	if tokenCount != 2 {
		t.Errorf("expected 2 verification tokens to exist (register + resend), got %d", tokenCount)
	}
}

func TestResendVerification_AlreadyVerified_SendsNothing(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	_, err := svc.Register(ctx, validInput("already-verified@example.com"))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	rawToken := m.lastToken(t)
	if err := svc.VerifyEmail(ctx, rawToken); err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	sentBefore := m.sentCount.Load()
	svc.ResendVerification(ctx, "already-verified@example.com")

	if m.sentCount.Load() != sentBefore {
		t.Errorf("expected no additional email for an already-verified user, sent before=%d after=%d", sentBefore, m.sentCount.Load())
	}
}

func TestRegister_WeakPassword_Rejected(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	svc := auth.NewUsecase(newTestStore(pool), &spyMailer{}, testLogger(), "http://localhost:3000", testTokenConfig())

	in := validInput("weak@example.com")
	in.Password = "short"

	_, err := svc.Register(ctx, in)
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for a too-short password, got: %v", err)
	}
}

// TestPassword_HashIsNeverPlaintext guards against the most damaging
// possible regression in this package: storing a recoverable password.
func TestPassword_HashIsNeverPlaintext(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	svc := auth.NewUsecase(newTestStore(pool), &spyMailer{}, testLogger(), "http://localhost:3000", testTokenConfig())

	in := validInput("hash-check@example.com")
	out, err := svc.Register(ctx, in)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	var storedHash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, out.UserID).Scan(&storedHash); err != nil {
		t.Fatalf("failed to read stored password hash: %v", err)
	}

	if storedHash == in.Password {
		t.Fatal("password_hash must never equal the plaintext password")
	}
	ok, err := password.Verify(in.Password, storedHash)
	if err != nil || !ok {
		t.Errorf("expected stored hash to verify against the original password, ok=%v err=%v", ok, err)
	}
}

func assertRowExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, id any) {
	t.Helper()
	var exists bool
	q := "SELECT EXISTS (SELECT 1 FROM " + table + " WHERE id = $1)"
	if err := pool.QueryRow(ctx, q, id).Scan(&exists); err != nil {
		t.Fatalf("failed to check row existence in %s: %v", table, err)
	}
	if !exists {
		t.Errorf("expected a row in %s for id=%v", table, id)
	}
}

func assertInvalidToken(t *testing.T, err error) {
	t.Helper()
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_token" {
		t.Fatalf("expected invalid_token DomainError, got: %v", err)
	}
}
