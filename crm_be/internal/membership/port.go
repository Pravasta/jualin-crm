package membership

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is membership's own table interface — the concrete
// implementation (postgresRepository, repository_postgres.go) satisfies
// it, but Usecase depends only on this interface (ADR-011), which is
// what makes usecase_unit_test.go's fake possible without Docker.
type Repository interface {
	Create(ctx context.Context, t tenant.Context, id, userID uuid.UUID, role tenant.Role) (*Membership, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Membership, error)
	FindActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*Membership, error)
	FindAllByOrgWithUser(ctx context.Context, t tenant.Context) ([]*MemberWithUser, error)
	UpdateRole(ctx context.Context, t tenant.Context, id uuid.UUID, role tenant.Role) error
	Deactivate(ctx context.Context, t tenant.Context, id uuid.UUID) error
	CountActiveOwners(ctx context.Context, t tenant.Context) (int, error)
}

// AuditRepository is declared locally (not imported from internal/auditlog)
// per ADR-011 — the interface belongs to the consumer.
type AuditRepository interface {
	Record(ctx context.Context, t tenant.Context, actorMembershipID *uuid.UUID, action string) error
}

// RefreshTokenRepository is the one capability Usecase.Deactivate needs
// from refresh_tokens — satisfied by auth.NewRefreshTokenRevoker's
// return value at the composition root. This package never imports
// internal/auth; see that package's RefreshTokenRevoker doc comment.
type RefreshTokenRepository interface {
	RevokeAllByMembershipID(ctx context.Context, membershipID uuid.UUID) error
}

// Repos bundles every repository a single Usecase call needs.
type Repos struct {
	Member       Repository
	Audit        AuditRepository
	RefreshToken RefreshTokenRepository
}

// Store is the Unit of Work Usecase depends on — same shape as auth.Store.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
