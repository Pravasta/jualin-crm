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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/mailer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/password"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/token"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

const minPasswordLength = 12

type Service struct {
	pool    *pgxpool.Pool
	mailer  mailer.Mailer
	logger  *slog.Logger
	baseURL string
}

func NewService(pool *pgxpool.Pool, m mailer.Mailer, logger *slog.Logger, baseURL string) *Service {
	return &Service{pool: pool, mailer: m, logger: logger, baseURL: baseURL}
}

type RegisterInput struct {
	OrganizationName string
	FullName         string
	Email            string
	Password         string
}

type RegisterOutput struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}

// Register creates an organization, its owner user, the owner
// membership, and a free subscription in one transaction (freeze bagian
// A1) — a failure at any point leaves nothing behind. The verification
// email is sent only after that transaction commits (Rule #32); a send
// failure is logged but never rolls back the registration.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*RegisterOutput, error) {
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

	txErr := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		orgRepo := organization.New(tx)
		userRepo := user.New(tx)
		membershipRepo := membership.New(tx)
		subRepo := subscription.New(tx)
		verifyRepo := newVerificationRepository(tx)
		auditRepo := auditlog.New(tx)

		if _, err := orgRepo.Create(ctx, orgID, in.OrganizationName); err != nil {
			return err
		}

		u, err := userRepo.Create(ctx, userID, in.Email, hash, in.FullName)
		if err != nil {
			return err // may be user.ErrEmailTaken — translated after InTx returns
		}
		createdUser = u

		t := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser}

		if _, err := membershipRepo.Create(ctx, t, membershipID, u.ID, tenant.RoleOwner); err != nil {
			return err
		}
		if _, err := subRepo.CreateFree(ctx, t, subID); err != nil {
			return err
		}
		if err := verifyRepo.Create(ctx, verificationID, u.ID, tokenHash); err != nil {
			return err
		}
		if err := auditRepo.Record(ctx, t, &membershipID, "user.registered"); err != nil {
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

	s.sendVerificationEmail(ctx, createdUser.Email, rawToken)

	return &RegisterOutput{UserID: createdUser.ID, OrganizationID: orgID}, nil
}

// VerifyEmail consumes a verification token. Missing, expired, and
// already-used tokens are all indistinguishable at the repository layer
// (httpx.ErrNotFound) and are translated to the same invalid_token
// response here — a client can't use response differences to enumerate
// which case applies.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	hash := token.Hash(rawToken)

	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		verifyRepo := newVerificationRepository(tx)
		userRepo := user.New(tx)
		membershipRepo := membership.New(tx)
		auditRepo := auditlog.New(tx)

		vt, err := verifyRepo.FindValidByHash(ctx, hash)
		if err != nil {
			if errors.Is(err, httpx.ErrNotFound) {
				return invalidTokenError()
			}
			return err
		}

		if err := verifyRepo.MarkUsed(ctx, vt.ID); err != nil {
			return err
		}
		if err := userRepo.MarkEmailVerified(ctx, vt.UserID); err != nil {
			return err
		}

		// A freshly registered user has exactly one membership — audit
		// logging needs an organization to attach to. This assumption
		// holds through Phase 1; revisit once #11 makes multi-org
		// membership common at verification time.
		memberships, err := membershipRepo.FindActiveByUserID(ctx, vt.UserID)
		if err != nil {
			return err
		}
		if len(memberships) > 0 {
			m := memberships[0]
			t := tenant.Context{OrganizationID: m.OrganizationID, PrincipalType: tenant.PrincipalUser}
			if err := auditRepo.Record(ctx, t, &m.ID, "email.verified"); err != nil {
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
func (s *Service) ResendVerification(ctx context.Context, email string) {
	u, err := user.New(s.pool).FindByEmail(ctx, email)
	if err != nil {
		return // unknown email — silently do nothing, still 202 to caller
	}
	if u.EmailVerifiedAt != nil {
		return // already verified — nothing to resend
	}

	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		s.logger.Error("failed to generate resend verification token", "err", err, "user_id", u.ID)
		return
	}

	verificationID := uuid.Must(uuid.NewV7())
	if err := newVerificationRepository(s.pool).Create(ctx, verificationID, u.ID, tokenHash); err != nil {
		s.logger.Error("failed to store resend verification token", "err", err, "user_id", u.ID)
		return
	}

	s.sendVerificationEmail(ctx, u.Email, rawToken)
}

func (s *Service) sendVerificationEmail(ctx context.Context, email, rawToken string) {
	link := fmt.Sprintf("%s/verify-email?token=%s", s.baseURL, rawToken)
	err := s.mailer.Send(ctx, mailer.Message{
		To:      email,
		Subject: "Verifikasi email Jualin CRM Anda",
		Body:    fmt.Sprintf("Klik tautan berikut untuk memverifikasi email Anda: %s\n\nTautan berlaku 24 jam.", link),
	})
	if err != nil {
		// Rule #32: send failure never rolls back work that already
		// committed. Logged structurally so it's visible, not silent.
		s.logger.Error("failed to send verification email", "err", err, "to", email)
	}
}

func invalidTokenError() error {
	return &httpx.DomainError{
		Status:  http.StatusBadRequest,
		Code:    "invalid_token",
		Message: "Token tidak valid atau sudah kedaluwarsa.",
	}
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
