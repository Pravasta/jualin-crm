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
	MarkEmailVerified(ctx context.Context, id uuid.UUID) error
}

type OrganizationRepository interface {
	Create(ctx context.Context, id uuid.UUID, name string) (*organization.Organization, error)
}

type MembershipRepository interface {
	Create(ctx context.Context, t tenant.Context, id, userID uuid.UUID, role tenant.Role) (*membership.Membership, error)
	FindActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*membership.Membership, error)
}

type SubscriptionRepository interface {
	CreateFree(ctx context.Context, t tenant.Context, id uuid.UUID) (*subscription.Subscription, error)
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

// Repos bundles every repository a single Register/VerifyEmail call
// needs. Store.InTx hands one of these — bound to a single transaction —
// to the closure that uses it.
type Repos struct {
	User   UserRepository
	Org    OrganizationRepository
	Member MembershipRepository
	Sub    SubscriptionRepository
	Verify VerificationTokenRepository
	Audit  AuditRepository
}

// Store is the Unit of Work Usecase depends on. It never appears as
// pgx.Tx anywhere in usecase.go — that type is confined to whatever
// implements Store (repository_postgres.go here, assembled at the
// composition root in cmd/api).
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos // non-transactional, for single reads (ResendVerification)
}
