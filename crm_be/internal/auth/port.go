package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

// Interfaces below are declared here — the consumer (this package's
// Usecase) — not by the packages that implement them (Rule #11, ADR-011).
// Each lists only the methods Usecase actually calls, not the full
// surface of user.Repository / organization.Repository / etc.
//
// *user.Repository, *organization.Repository, *membership.Repository, and
// *subscription.Repository already satisfy these via Go's implicit
// interfaces — internal/user, internal/organization, internal/membership,
// and internal/subscription never import internal/auth to make that true.
// Only internal/auth imports internal/user etc. here, for the *user.User
// (and sibling) return types — never for their concrete Repository types.

type UserRepository interface {
	Create(ctx context.Context, id uuid.UUID, email, passwordHash, fullName string) (*user.User, error)
	FindByEmail(ctx context.Context, email string) (*user.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*user.User, error)
	MarkEmailVerified(ctx context.Context, id uuid.UUID) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

type OrganizationRepository interface {
	Create(ctx context.Context, id uuid.UUID, name string) (*organization.Organization, error)
	FindByID(ctx context.Context, id uuid.UUID) (*organization.Organization, error)
}

type MembershipRepository interface {
	Create(ctx context.Context, t tenant.Context, id, userID uuid.UUID, role tenant.Role) (*membership.Membership, error)
	FindActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*membership.Membership, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*membership.Membership, error)
}

type SubscriptionRepository interface {
	CreateFree(ctx context.Context, t tenant.Context, id uuid.UUID) (*subscription.Subscription, error)
}

// PlanResolver is declared here, consumer-side (ADR-011), same shape as
// RefreshTokenRevoker below — Me (usecase.go) needs only this one
// computation from internal/subscription, not its Repository or
// RequireChannel. Satisfied structurally by
// subscription.NewUsecase(...)'s return value at the composition root.
type PlanResolver interface {
	ResolvePlan(ctx context.Context, t tenant.Context) (code string, channels map[string]bool, limits subscription.Limits, err error)
}

// LeadCounter and SeatCounter are the two meters GET /v1/me reports
// alongside the plan's limits (Phase 8.5 §7): what the organization has
// already used this month, resolved SERVER-side so no client ever
// computes it (D5 of Phase 8, applied to quantities).
//
// Declared here, consumer-side (ADR-011). *lead.Repository,
// *membership.Repository, and *invitation.Repository satisfy them
// structurally — those packages never import internal/auth.
//
// Seats deliberately takes TWO counts, not one: an invitation nobody has
// accepted yet still holds a seat (TD 8.5 §6). Summing them here means
// the displayed number matches the number #123's enforcement will
// refuse on — a mismatch there would read as a bug to the customer.
type LeadCounter interface {
	CountCreatedThisMonth(ctx context.Context, t tenant.Context) (int, error)
}

type SeatCounter interface {
	CountActive(ctx context.Context, t tenant.Context) (int, error)
}

type PendingSeatCounter interface {
	CountPendingSeats(ctx context.Context, t tenant.Context) (int, error)
}

// VerificationTokenRepository has no consumers outside this package —
// its implementation lives in repository_postgres.go, right alongside
// this interface, rather than in a separate internal/verification
// package that would exist for no other purpose.
type VerificationTokenRepository interface {
	Create(ctx context.Context, id, userID uuid.UUID, tokenHash string) error
	FindValidByHash(ctx context.Context, hash string) (*EmailVerificationToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
}

type AuditRepository interface {
	Record(ctx context.Context, t tenant.Context, actorMembershipID *uuid.UUID, action string) error
}

// RefreshTokenRepository has no consumers outside this package —
// refresh_tokens has no natural home elsewhere, unlike user/organization/
// membership/subscription.
//
// FindByHashForUpdate and RevokeAllByUserID deliberately do NOT take
// tenant.Context — the documented exception TD phase 1 §8 requires:
//   - FindByHashForUpdate is how a caller *discovers* which organization
//     a refresh token belongs to in the first place; requiring a
//     tenant.Context to look it up would be circular. The row it returns
//     carries OrganizationID, which the caller then uses to build a
//     tenant.Context for every subsequent action in the same request.
//   - RevokeAllByUserID intentionally acts across every organization a
//     user belongs to — password reset must kill every session
//     everywhere that user is logged in, not just the org they happen to
//     be resetting from.
type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *RefreshToken) error
	FindByHashForUpdate(ctx context.Context, hash string) (*RefreshToken, error)
	MarkReplaced(ctx context.Context, id, replacedByID uuid.UUID) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID) error
	RevokeByID(ctx context.Context, id uuid.UUID) error
	RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error
}

// RefreshTokenRevoker exists purely so the composition root can hand
// internal/membership the one capability it needs from refresh_tokens
// (revoke every session tied to a specific membership, on deactivation —
// TD phase 1 §2.3 rule #2) without membership importing internal/auth or
// internal/auth importing membership. membership/port.go declares its
// own one-method local interface; auth.NewRefreshTokenRevoker's return
// value satisfies it structurally. Not part of RefreshTokenRepository
// above because auth.Usecase itself never calls this method — ADR-011's
// "hanya method yang dipakai oleh consumer" applies per interface, not
// per implementation.
type RefreshTokenRevoker interface {
	RevokeAllByMembershipID(ctx context.Context, membershipID uuid.UUID) error
}

// PasswordResetTokenRepository has no consumers outside this package,
// same reasoning as VerificationTokenRepository.
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, id, userID uuid.UUID, tokenHash string) error
	FindValidByHash(ctx context.Context, hash string) (*PasswordResetToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
}

// Repos bundles every repository a single Usecase call needs. Store.InTx
// hands one of these — bound to a single transaction — to the closure
// that uses it.
type Repos struct {
	User         UserRepository
	Org          OrganizationRepository
	Member       MembershipRepository
	Sub          SubscriptionRepository
	Plan         PlanResolver
	LeadCount    LeadCounter
	SeatCount    SeatCounter
	PendingSeats PendingSeatCounter
	Verify       VerificationTokenRepository
	Audit        AuditRepository
	RefreshToken RefreshTokenRepository
	ResetToken   PasswordResetTokenRepository
}

// Store is the Unit of Work Usecase depends on. It never appears as
// pgx.Tx anywhere in usecase.go — that type is confined to whatever
// implements Store (repository_postgres.go here, assembled at the
// composition root in cmd/api).
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos // non-transactional, for single reads (ResendVerification)
}
