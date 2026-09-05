// Package auth orchestrates the flows that create and verify identity:
// registration (atomic across four tables), email verification, and
// resend. Login, refresh, and password reset are issue #10; RBAC,
// invitations, and membership deactivation are issue #11.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/mailer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/password"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/token"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

const minPasswordLength = 12

// Usecase depends only on Store (port.go) and mailer.Mailer — never on
// *pgxpool.Pool or pgx.Tx directly (ADR-011). That's what makes
// TestRegister_WeakPassword_Rejected-style tests possible with a fake
// Store and no Docker.
type Usecase struct {
	store   Store
	mailer  mailer.Mailer
	logger  *slog.Logger
	baseURL string
	tokens  TokenConfig
}

func NewUsecase(store Store, m mailer.Mailer, logger *slog.Logger, baseURL string, tokens TokenConfig) *Usecase {
	return &Usecase{store: store, mailer: m, logger: logger, baseURL: baseURL, tokens: tokens}
}

// Register creates an organization, its owner user, the owner
// membership, and a free subscription in one transaction (freeze bagian
// A1) — a failure at any point leaves nothing behind. The verification
// email is sent only after that transaction commits (Rule #32); a send
// failure is logged but never rolls back the registration.
func (u *Usecase) Register(ctx context.Context, in RegisterInput) (*RegisterOutput, error) {
	if verr := validateRegisterInput(in); verr != nil {
		return nil, verr
	}

	hash, err := password.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("auth: register: hash password: %w", err)
	}

	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("auth: register: generate verification token: %w", err)
	}

	orgID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	membershipID := uuid.Must(uuid.NewV7())
	subID := uuid.Must(uuid.NewV7())
	verificationID := uuid.Must(uuid.NewV7())

	var createdUser *user.User

	txErr := u.store.InTx(ctx, func(r Repos) error {
		if _, err := r.Org.Create(ctx, orgID, in.OrganizationName); err != nil {
			return err
		}

		created, err := r.User.Create(ctx, userID, in.Email, hash, in.FullName)
		if err != nil {
			return err // may be user.ErrEmailTaken — translated after InTx returns
		}
		createdUser = created

		t := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser}

		if _, err := r.Member.Create(ctx, t, membershipID, created.ID, tenant.RoleOwner); err != nil {
			return err
		}
		if _, err := r.Sub.CreateFree(ctx, t, subID); err != nil {
			return err
		}
		if err := r.Verify.Create(ctx, verificationID, created.ID, tokenHash); err != nil {
			return err
		}
		if err := r.Audit.Record(ctx, t, &membershipID, "user.registered"); err != nil {
			return err
		}

		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, user.ErrEmailTaken) {
			return nil, &httpx.DomainError{
				Status:  http.StatusConflict,
				Code:    "email_already_registered",
				Message: "Email sudah terdaftar.",
			}
		}
		return nil, fmt.Errorf("auth: register: %w", txErr)
	}

	u.sendVerificationEmail(ctx, createdUser.Email, rawToken)

	return &RegisterOutput{UserID: createdUser.ID, OrganizationID: orgID}, nil
}

// VerifyEmail consumes a verification token. Missing, expired, and
// already-used tokens are all indistinguishable at the repository layer
// (httpx.ErrNotFound) and are translated to the same invalid_token
// response here — a client can't use response differences to enumerate
// which case applies.
func (u *Usecase) VerifyEmail(ctx context.Context, rawToken string) error {
	hash := token.Hash(rawToken)

	return u.store.InTx(ctx, func(r Repos) error {
		vt, err := r.Verify.FindValidByHash(ctx, hash)
		if err != nil {
			if errors.Is(err, httpx.ErrNotFound) {
				return invalidTokenError()
			}
			return err
		}

		if err := r.Verify.MarkUsed(ctx, vt.ID); err != nil {
			return err
		}
		if err := r.User.MarkEmailVerified(ctx, vt.UserID); err != nil {
			return err
		}

		// A freshly registered user has exactly one membership — audit
		// logging needs an organization to attach to. This assumption
		// holds through Phase 1; revisit once #11 makes multi-org
		// membership common at verification time.
		memberships, err := r.Member.FindActiveByUserID(ctx, vt.UserID)
		if err != nil {
			return err
		}
		if len(memberships) > 0 {
			m := memberships[0]
			t := tenant.Context{OrganizationID: m.OrganizationID, PrincipalType: tenant.PrincipalUser}
			if err := r.Audit.Record(ctx, t, &m.ID, "email.verified"); err != nil {
				return err
			}
		}

		return nil
	})
}

// ResendVerification always returns nil (202) regardless of whether the
// email exists or is already verified — anti-enumeration (Rule: TD §6.3).
// An email is only actually sent when there's a real, unverified account
// to send it to.
func (u *Usecase) ResendVerification(ctx context.Context, email string) {
	repos := u.store.Repos()

	usr, err := repos.User.FindByEmail(ctx, email)
	if err != nil {
		return // unknown email — silently do nothing, still 202 to caller
	}
	if usr.EmailVerifiedAt != nil {
		return // already verified — nothing to resend
	}

	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		u.logger.Error("failed to generate resend verification token", "err", err, "user_id", usr.ID)
		return
	}

	verificationID := uuid.Must(uuid.NewV7())
	if err := repos.Verify.Create(ctx, verificationID, usr.ID, tokenHash); err != nil {
		u.logger.Error("failed to store resend verification token", "err", err, "user_id", usr.ID)
		return
	}

	u.sendVerificationEmail(ctx, usr.Email, rawToken)
}

func (u *Usecase) sendVerificationEmail(ctx context.Context, email, rawToken string) {
	link := fmt.Sprintf("%s/verify-email?token=%s", u.baseURL, rawToken)
	err := u.mailer.Send(ctx, mailer.Message{
		To:      email,
		Subject: "Verifikasi email Jualin CRM Anda",
		Body:    fmt.Sprintf("Klik tautan berikut untuk memverifikasi email Anda: %s\n\nTautan berlaku 24 jam.", link),
	})
	if err != nil {
		// Rule #32: send failure never rolls back work that already
		// committed. Logged structurally so it's visible, not silent.
		u.logger.Error("failed to send verification email", "err", err, "to", email)
	}
}

// Login authenticates a user and issues a new access+refresh token pair.
// A wrong email, wrong password, or an OrganizationID that doesn't match
// any of the user's active memberships all collapse to the same
// invalid_credentials response — none of them should let a caller tell
// which part was wrong. Failed attempts are never audited (no tenant yet,
// TD §12) — they go to the application logger instead.
func (u *Usecase) Login(ctx context.Context, in LoginInput) (*LoginOutput, error) {
	if verr := validateLoginInput(in); verr != nil {
		return nil, verr
	}

	usr, err := u.store.Repos().User.FindByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, httpx.ErrNotFound) {
			u.logger.Warn("login failed: unknown email")
			return nil, invalidCredentialsError()
		}
		return nil, fmt.Errorf("auth: login: find user: %w", err)
	}

	ok, err := password.Verify(in.Password, usr.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("auth: login: verify password: %w", err)
	}
	if !ok {
		u.logger.Warn("login failed: wrong password", "user_id", usr.ID)
		return nil, invalidCredentialsError()
	}

	if usr.EmailVerifiedAt == nil {
		return nil, &httpx.DomainError{
			Status:  http.StatusForbidden,
			Code:    "email_not_verified",
			Message: "Email belum terverifikasi.",
		}
	}

	memberships, err := u.store.Repos().Member.FindActiveByUserID(ctx, usr.ID)
	if err != nil {
		return nil, fmt.Errorf("auth: login: find memberships: %w", err)
	}
	if len(memberships) == 0 {
		u.logger.Warn("login failed: no active membership", "user_id", usr.ID)
		return nil, invalidCredentialsError()
	}

	selected, err := selectMembership(memberships, in.OrganizationID)
	if err != nil {
		return nil, err
	}
	if selected == nil {
		// >1 membership and the caller hasn't picked one yet (TD §6.2).
		return nil, u.organizationSelectionError(ctx, memberships)
	}

	out, txErr := u.issueSession(ctx, usr.ID, selected.OrganizationID, selected.ID, selected.Role, in.Client, "auth.login")
	if txErr != nil {
		return nil, fmt.Errorf("auth: login: %w", txErr)
	}
	return out, nil
}

// selectMembership returns the membership to log into, or nil (not an
// error) when the caller must be asked to choose. orgID non-nil must
// match one of memberships — a mismatch is treated identically to wrong
// credentials, not a distinct error, so a caller can't use it to probe
// which organizations a given email belongs to.
func selectMembership(memberships []*membership.Membership, orgID *uuid.UUID) (*membership.Membership, error) {
	if orgID == nil {
		if len(memberships) == 1 {
			return memberships[0], nil
		}
		return nil, nil
	}
	for _, m := range memberships {
		if m.OrganizationID == *orgID {
			return m, nil
		}
	}
	return nil, invalidCredentialsError()
}

func (u *Usecase) organizationSelectionError(ctx context.Context, memberships []*membership.Membership) error {
	opts := make([]OrgOption, 0, len(memberships))
	for _, m := range memberships {
		org, err := u.store.Repos().Org.FindByID(ctx, m.OrganizationID)
		if err != nil {
			return fmt.Errorf("auth: login: load organization for selection: %w", err)
		}
		opts = append(opts, OrgOption{ID: org.ID, Name: org.Name})
	}
	return &OrganizationSelectionError{Organizations: opts}
}

// issueSession is shared by Login and the "issue a fresh pair" half of
// Refresh's rotation — both create a refresh_tokens row and (optionally)
// an audit entry inside one transaction.
func (u *Usecase) issueSession(
	ctx context.Context,
	userID, orgID, membershipID uuid.UUID,
	role tenant.Role,
	client, auditAction string,
) (*LoginOutput, error) {
	accessTTL := u.tokens.AccessTokenTTL
	refreshTTL := refreshTTLFor(u.tokens, client)

	access, err := accesstoken.Issue(u.tokens.JWTSecret, accessTTL, userID, orgID, membershipID, role)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	rawRefresh, refreshHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	rt := &RefreshToken{
		ID:             uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		MembershipID:   membershipID,
		TokenHash:      refreshHash,
		FamilyID:       uuid.Must(uuid.NewV7()),
		Client:         client,
		ExpiresAt:      time.Now().Add(refreshTTL),
	}

	txErr := u.store.InTx(ctx, func(r Repos) error {
		if err := r.RefreshToken.Create(ctx, rt); err != nil {
			return err
		}
		if auditAction != "" {
			t := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser}
			if err := r.Audit.Record(ctx, t, &membershipID, auditAction); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	return &LoginOutput{
		AccessToken:     access,
		RefreshToken:    rawRefresh,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		Client:          client,
		OrganizationID:  orgID,
		MembershipID:    membershipID,
		Role:            role,
	}, nil
}

func refreshTTLFor(cfg TokenConfig, client string) time.Duration {
	if client == ClientMobile {
		return cfg.RefreshTokenTTLMobile
	}
	return cfg.RefreshTokenTTLDashboard
}

// Refresh rotates a refresh token: the presented token is marked
// replaced and a new one takes its place, in the same family. Reusing a
// token that was already rotated (or already revoked) is the reuse-
// detection signal from TD §4 — every token in that family is revoked
// and the caller gets the same 401 a not-found/expired token would, so a
// legitimate client and an attacker racing the same stolen token can't
// tell which of them "won".
func (u *Usecase) Refresh(ctx context.Context, rawToken string) (*RefreshOutput, error) {
	hash := token.Hash(rawToken)

	var (
		reused bool
		out    *RefreshOutput
	)

	txErr := u.store.InTx(ctx, func(r Repos) error {
		rt, err := r.RefreshToken.FindByHashForUpdate(ctx, hash)
		if err != nil {
			if errors.Is(err, httpx.ErrNotFound) {
				return invalidCredentialsError()
			}
			return err
		}

		if rt.RevokedAt != nil || rt.ReplacedByID != nil {
			reused = true
			t := tenant.Context{OrganizationID: rt.OrganizationID, PrincipalType: tenant.PrincipalUser}
			if err := r.RefreshToken.RevokeFamily(ctx, rt.FamilyID); err != nil {
				return err
			}
			return r.Audit.Record(ctx, t, &rt.MembershipID, "auth.refresh_reused")
		}

		if !rt.ExpiresAt.After(time.Now()) {
			return invalidCredentialsError()
		}

		// role/userID aren't stored on refresh_tokens (only
		// organization_id + membership_id, TD §1) — reload the
		// membership to put an accurate claim set in the new access
		// token.
		memberT := tenant.Context{OrganizationID: rt.OrganizationID, PrincipalType: tenant.PrincipalUser}
		mem, err := r.Member.FindByID(ctx, memberT, rt.MembershipID)
		if err != nil {
			return err
		}

		access, err := accesstoken.Issue(u.tokens.JWTSecret, u.tokens.AccessTokenTTL, mem.UserID, rt.OrganizationID, rt.MembershipID, mem.Role)
		if err != nil {
			return fmt.Errorf("issue access token: %w", err)
		}

		rawRefresh, refreshHash, err := token.Generate()
		if err != nil {
			return fmt.Errorf("generate refresh token: %w", err)
		}

		newRT := &RefreshToken{
			ID:             uuid.Must(uuid.NewV7()),
			OrganizationID: rt.OrganizationID,
			MembershipID:   rt.MembershipID,
			TokenHash:      refreshHash,
			FamilyID:       rt.FamilyID,
			Client:         rt.Client,
			ExpiresAt:      time.Now().Add(refreshTTLFor(u.tokens, rt.Client)),
		}
		if err := r.RefreshToken.Create(ctx, newRT); err != nil {
			return err
		}
		if err := r.RefreshToken.MarkReplaced(ctx, rt.ID, newRT.ID); err != nil {
			return err
		}

		out = &RefreshOutput{
			AccessToken:     access,
			RefreshToken:    rawRefresh,
			AccessTokenTTL:  u.tokens.AccessTokenTTL,
			RefreshTokenTTL: time.Until(newRT.ExpiresAt),
			Client:          rt.Client,
		}
		return nil
	})
	if reused {
		return nil, invalidCredentialsError()
	}
	if txErr != nil {
		return nil, fmt.Errorf("auth: refresh: %w", txErr)
	}
	return out, nil
}

// Logout revokes the presented refresh token. A token that doesn't
// exist (already logged out, already expired and cleaned up) is treated
// as success — the caller asked to not be logged in anymore, and that's
// already true, so there is nothing to distinguish through the response.
func (u *Usecase) Logout(ctx context.Context, rawToken string) error {
	hash := token.Hash(rawToken)

	return u.store.InTx(ctx, func(r Repos) error {
		rt, err := r.RefreshToken.FindByHashForUpdate(ctx, hash)
		if err != nil {
			if errors.Is(err, httpx.ErrNotFound) {
				return nil
			}
			return err
		}
		if err := r.RefreshToken.RevokeByID(ctx, rt.ID); err != nil {
			return err
		}
		t := tenant.Context{OrganizationID: rt.OrganizationID, PrincipalType: tenant.PrincipalUser}
		return r.Audit.Record(ctx, t, &rt.MembershipID, "auth.logout")
	})
}

// ForgotPassword always behaves the same externally regardless of
// whether the email exists — same anti-enumeration shape as
// ResendVerification (TD §6.3).
func (u *Usecase) ForgotPassword(ctx context.Context, email string) {
	repos := u.store.Repos()

	usr, err := repos.User.FindByEmail(ctx, email)
	if err != nil {
		return
	}

	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		u.logger.Error("failed to generate password reset token", "err", err, "user_id", usr.ID)
		return
	}

	resetID := uuid.Must(uuid.NewV7())
	if err := repos.ResetToken.Create(ctx, resetID, usr.ID, tokenHash); err != nil {
		u.logger.Error("failed to store password reset token", "err", err, "user_id", usr.ID)
		return
	}

	// Audited against the user's first active membership — same "holds
	// through Phase 1" assumption VerifyEmail documents; revisited at #11.
	if memberships, err := repos.Member.FindActiveByUserID(ctx, usr.ID); err == nil && len(memberships) > 0 {
		m := memberships[0]
		t := tenant.Context{OrganizationID: m.OrganizationID, PrincipalType: tenant.PrincipalUser}
		if err := repos.Audit.Record(ctx, t, &m.ID, "password.reset_requested"); err != nil {
			u.logger.Error("failed to record password reset audit", "err", err, "user_id", usr.ID)
		}
	}

	link := fmt.Sprintf("%s/reset-password?token=%s", u.baseURL, rawToken)
	err = u.mailer.Send(ctx, mailer.Message{
		To:      usr.Email,
		Subject: "Reset password Jualin CRM Anda",
		Body:    fmt.Sprintf("Klik tautan berikut untuk mengatur ulang password Anda: %s\n\nTautan berlaku 1 jam.", link),
	})
	if err != nil {
		u.logger.Error("failed to send password reset email", "err", err, "to", usr.Email)
	}
}

// ResetPassword consumes a reset token, sets a new password, and revokes
// every refresh token the user holds across every organization (TD §6:
// "mencabut seluruh sesi user") — a stolen password is exactly the
// moment every existing session becomes suspect, not just the one on the
// device where the reset happened.
func (u *Usecase) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < minPasswordLength {
		return httpx.NewValidationError(httpx.ErrorDetail{Field: "password", Code: "too_short"})
	}

	newHash, err := password.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("auth: reset password: hash password: %w", err)
	}

	hash := token.Hash(rawToken)

	return u.store.InTx(ctx, func(r Repos) error {
		rt, err := r.ResetToken.FindValidByHash(ctx, hash)
		if err != nil {
			if errors.Is(err, httpx.ErrNotFound) {
				return invalidTokenError()
			}
			return err
		}

		if err := r.ResetToken.MarkUsed(ctx, rt.ID); err != nil {
			return err
		}
		if err := r.User.UpdatePassword(ctx, rt.UserID, newHash); err != nil {
			return err
		}
		if err := r.RefreshToken.RevokeAllByUserID(ctx, rt.UserID); err != nil {
			return err
		}

		memberships, err := r.Member.FindActiveByUserID(ctx, rt.UserID)
		if err != nil {
			return err
		}
		for _, m := range memberships {
			t := tenant.Context{OrganizationID: m.OrganizationID, PrincipalType: tenant.PrincipalUser}
			if err := r.Audit.Record(ctx, t, &m.ID, "password.reset_completed"); err != nil {
				return err
			}
		}
		return nil
	})
}

// ParseAccessToken verifies raw and returns its claims. Handler-side
// middleware calls this rather than accesstoken.Parse directly so the
// JWT secret has exactly one owner (Usecase, via TokenConfig) — the
// handler never touches it.
func (u *Usecase) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse(u.tokens.JWTSecret, raw)
}

// Me loads the display data for GET /v1/me. It trusts t (already
// verified by AuthMiddleware from the access token's signed claims) —
// re-checking membership status on every request is #11's job
// (deactivation enforcement, TD §17), not this issue's.
func (u *Usecase) Me(ctx context.Context, t tenant.Context) (*MeOutput, error) {
	repos := u.store.Repos()

	usr, err := repos.User.FindByID(ctx, *t.UserID)
	if err != nil {
		return nil, fmt.Errorf("auth: me: load user: %w", err)
	}
	org, err := repos.Org.FindByID(ctx, t.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("auth: me: load organization: %w", err)
	}
	planCode, planChannels, limits, err := repos.Plan.ResolvePlan(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("auth: me: resolve plan: %w", err)
	}

	// Two meters, three queries — the cost this endpoint pays so that no
	// client ever computes usage itself (Phase 8.5 §7). If this turns out
	// to be measurably expensive (it is called on EVERY protected screen
	// via SessionGate), the documented first move is to keep limits here
	// and serve usage from the Langganan screen's own endpoint instead.
	leadsThisMonth, err := repos.LeadCount.CountCreatedThisMonth(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("auth: me: count leads this month: %w", err)
	}
	activeSeats, err := repos.SeatCount.CountActive(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("auth: me: count active seats: %w", err)
	}
	pendingSeats, err := repos.PendingSeats.CountPendingSeats(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("auth: me: count pending seats: %w", err)
	}

	return &MeOutput{
		UserID:           usr.ID,
		Email:            usr.Email,
		FullName:         usr.FullName,
		OrganizationID:   org.ID,
		OrganizationName: org.Name,
		MembershipID:     *t.MembershipID,
		Role:             t.Role,
		PlanCode:         planCode,
		PlanChannels:     planChannels,
		PlanLimits: PlanLimits{
			LeadsPerMonth: limits.LeadsPerMonth,
			Seats:         limits.Seats,
		},
		PlanUsage: PlanUsage{
			LeadsThisMonth: leadsThisMonth,
			SeatsUsed:      activeSeats + pendingSeats,
		},
	}, nil
}

func invalidCredentialsError() error {
	return &httpx.DomainError{
		Status:  http.StatusUnauthorized,
		Code:    "invalid_credentials",
		Message: "Email atau password salah.",
	}
}

func invalidTokenError() error {
	return &httpx.DomainError{
		Status:  http.StatusBadRequest,
		Code:    "invalid_token",
		Message: "Token tidak valid atau sudah kedaluwarsa.",
	}
}

func validateLoginInput(in LoginInput) error {
	var details []httpx.ErrorDetail
	if !looksLikeEmail(in.Email) {
		details = append(details, httpx.ErrorDetail{Field: "email", Code: "invalid_format"})
	}
	if in.Password == "" {
		details = append(details, httpx.ErrorDetail{Field: "password", Code: "required"})
	}
	if in.Client != ClientDashboard && in.Client != ClientMobile {
		details = append(details, httpx.ErrorDetail{Field: "client", Code: "invalid_value"})
	}
	if len(details) > 0 {
		return httpx.NewValidationError(details...)
	}
	return nil
}

func validateRegisterInput(in RegisterInput) error {
	var details []httpx.ErrorDetail
	if in.OrganizationName == "" {
		details = append(details, httpx.ErrorDetail{Field: "organization_name", Code: "required"})
	}
	if in.FullName == "" {
		details = append(details, httpx.ErrorDetail{Field: "full_name", Code: "required"})
	}
	if !looksLikeEmail(in.Email) {
		details = append(details, httpx.ErrorDetail{Field: "email", Code: "invalid_format"})
	}
	if len(in.Password) < minPasswordLength {
		details = append(details, httpx.ErrorDetail{Field: "password", Code: "too_short"})
	}
	if len(details) > 0 {
		return httpx.NewValidationError(details...)
	}
	return nil
}

func looksLikeEmail(s string) bool {
	at := -1
	for i, c := range s {
		if c == '@' {
			at = i
			break
		}
	}
	return at > 0 && at < len(s)-1
}
