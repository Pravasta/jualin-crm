package auth

import (
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// EmailVerificationToken is global, like user.User — it belongs to a
// user_id, not an organization. See email_verification_tokens in
// migration 0002. Unlike user/organization/membership, this entity has
// no consumers outside internal/auth, so its interface (port.go) and
// implementation (repository_postgres.go) both live here.
type EmailVerificationToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

const emailVerificationTTL = 24 * time.Hour

// RegisterInput is Usecase.Register's argument — a usecase-level type,
// not a database entity, but it lives here alongside EmailVerificationToken
// as this package's domain vocabulary.
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

// PasswordResetToken mirrors EmailVerificationToken's shape — same
// lifetime rules, kept as a distinct type (and distinct table) because
// accepting a reset token where a verification token was expected (or
// vice versa) must be structurally impossible, not merely unlikely
// (TD phase 1 §1).
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

const passwordResetTTL = time.Hour

// RefreshToken is the session row backing the opaque refresh token TD
// phase 1 §4 describes. FamilyID links every token produced by rotating
// a single original login together — reuse of any non-current member of
// a family revokes the whole family (TD §4 "Deteksi penggunaan ulang").
type RefreshToken struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	MembershipID   uuid.UUID
	TokenHash      string
	FamilyID       uuid.UUID
	Client         string // "dashboard" | "mobile"
	ExpiresAt      time.Time
	RevokedAt      *time.Time
	ReplacedByID   *uuid.UUID
	CreatedAt      time.Time
}

const (
	ClientDashboard = "dashboard"
	ClientMobile    = "mobile"
)

// TokenConfig carries the token-issuing settings Usecase needs. Grouped
// into one struct (rather than four constructor params) because they
// always travel together from config.Config to NewUsecase.
type TokenConfig struct {
	JWTSecret                []byte
	AccessTokenTTL           time.Duration
	RefreshTokenTTLDashboard time.Duration
	RefreshTokenTTLMobile    time.Duration
}

// LoginInput is Usecase.Login's argument. OrganizationID is set on the
// second call of a two-step login — see OrganizationSelectionError.
type LoginInput struct {
	Email          string
	Password       string
	Client         string
	OrganizationID *uuid.UUID
}

// LoginOutput carries both possible response shapes (cookie vs body) —
// the handler decides which fields it actually serializes based on
// Client, per TD phase 1 §5's acceptance criteria (a dashboard response
// must contain no token fields at all).
type LoginOutput struct {
	AccessToken     string
	RefreshToken    string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Client          string
	OrganizationID  uuid.UUID
	MembershipID    uuid.UUID
	Role            tenant.Role
}

// RefreshOutput is Usecase.Refresh's result — same shape as LoginOutput
// for the same handler-decides-serialization reason.
type RefreshOutput struct {
	AccessToken     string
	RefreshToken    string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Client          string
}

// MeOutput is GET /v1/me's payload.
type MeOutput struct {
	UserID           uuid.UUID
	Email            string
	FullName         string
	OrganizationID   uuid.UUID
	OrganizationName string
	MembershipID     uuid.UUID
	Role             tenant.Role
}

// OrgOption is one entry in OrganizationSelectionError's list.
type OrgOption struct {
	ID   uuid.UUID
	Name string
}

// OrganizationSelectionError signals TD phase 1 §6.2: a user with more
// than one active membership must pick which organization to log into.
// It carries an `organizations` field the standard httpx.DomainError
// can't express — api.md documents this as a deliberate, narrow
// extension of the error envelope, not a precedent for arbitrary error
// payloads.
type OrganizationSelectionError struct {
	Organizations []OrgOption
}

func (e *OrganizationSelectionError) Error() string {
	return "organization selection required"
}
