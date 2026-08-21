package invitation

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

// Repository is invitations' own table interface. FindValidByHash
// deliberately does NOT take tenant.Context — same documented exception
// as auth's refresh token lookup: which organization an invitation
// belongs to is exactly what resolving the hash tells the caller, not
// something known beforehand (TD phase 1 §8).
type Repository interface {
	Create(ctx context.Context, inv *Invitation) error
	FindByOrgPending(ctx context.Context, t tenant.Context) ([]*Invitation, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Invitation, error)
	FindValidByHash(ctx context.Context, hash string) (*Invitation, error)
	MarkAccepted(ctx context.Context, id uuid.UUID) error
	MarkRevoked(ctx context.Context, id uuid.UUID) error
}

// UserRepository/MembershipRepository/OrganizationRepository are declared
// here (the consumer) per ADR-011 — each lists only what Usecase calls.
// *user.Repository, *membership.postgresRepository (via membership.New),
// and *organization.Repository already satisfy these structurally.
type UserRepository interface {
	Create(ctx context.Context, id uuid.UUID, email, passwordHash, fullName string) (*user.User, error)
	FindByEmail(ctx context.Context, email string) (*user.User, error)
	MarkEmailVerified(ctx context.Context, id uuid.UUID) error
}

type MembershipRepository interface {
	Create(ctx context.Context, t tenant.Context, id, userID uuid.UUID, role tenant.Role) (*membership.Membership, error)
}

type OrganizationRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*organization.Organization, error)
}

type AuditRepository interface {
	Record(ctx context.Context, t tenant.Context, actorMembershipID *uuid.UUID, action string) error
}

type Repos struct {
	User       UserRepository
	Member     MembershipRepository
	Org        OrganizationRepository
	Audit      AuditRepository
	Invitation Repository
}

// Store is the Unit of Work Usecase depends on — same shape as auth.Store.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
