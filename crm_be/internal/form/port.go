package form

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is form's own table interface.
type Repository interface {
	Create(ctx context.Context, t tenant.Context, f *Form) error
	FindByOrg(ctx context.Context, t tenant.Context) ([]*Form, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Form, error)
	Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Form, error)
	Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error

	// FindByPublicKey deliberately does NOT take tenant.Context — same
	// documented exception as apikey.Repository.FindByKeyID (Phase 4
	// #46), itself following invitation.Repository.FindValidByHash and
	// RefreshTokenRepository.FindByHashForUpdate (Phase 1): which
	// organization a form belongs to is exactly what resolving
	// public_key tells the caller, not something known beforehand (Rule
	// #5). No HTTP path calls this in #85 — it's exercised directly by
	// repository_test.go (including an EXPLAIN proving it's an index
	// hit) and exists now because #87's ResolvePublicKey needs the
	// exact same interface, not a reshaped one added mid-phase.
	FindByPublicKey(ctx context.Context, publicKey string) (*Form, error)
}

// AuditRepository is declared here (the consumer) per ADR-011 —
// *auditlog.Repository already satisfies it structurally.
type AuditRepository interface {
	Record(ctx context.Context, t tenant.Context, actorMembershipID *uuid.UUID, action string) error
}

type Repos struct {
	Form  Repository
	Audit AuditRepository
}

// Store is the Unit of Work Usecase depends on — same shape as every
// other domain's Store since ADR-011.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
