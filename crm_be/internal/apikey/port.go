package apikey

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is api_key's own table interface.
type Repository interface {
	Create(ctx context.Context, t tenant.Context, k *APIKey) error
	FindByOrg(ctx context.Context, t tenant.Context) ([]*APIKey, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*APIKey, error)
	Revoke(ctx context.Context, t tenant.Context, id uuid.UUID) error

	// FindByKeyID deliberately does NOT take tenant.Context — same
	// documented exception as invitation.Repository.FindValidByHash and
	// RefreshTokenRepository.FindByHashForUpdate (Phase 1): which
	// organization a key belongs to is exactly what resolving key_id
	// tells the caller, not something known beforehand (Rule #5). No
	// HTTP path calls this in #46 — it's exercised directly by
	// repository_test.go (including an EXPLAIN proving it's an index
	// hit) and exists now because #47's authn.APIKeyResolver needs the
	// exact same interface, not a reshaped one added mid-phase.
	FindByKeyID(ctx context.Context, keyID string) (*APIKey, error)

	// TouchLastUsed writes last_used_at = now() for id. No tenant.Context
	// either, for the same reason FindByKeyID has none — by the time
	// this is called, id already came from a successful FindByKeyID, so
	// scoping to an organization would be redundant, not safer. Called
	// from Usecase.ResolveAPIKey (#47, TD §10), throttled to at most
	// once per 5 minutes per key — ADR-004 aturan #3 explicitly warns
	// against writing this on every request.
	TouchLastUsed(ctx context.Context, id uuid.UUID) error
}

// AuditRepository is declared here (the consumer) per ADR-011 —
// *auditlog.Repository already satisfies it structurally.
type AuditRepository interface {
	Record(ctx context.Context, t tenant.Context, actorMembershipID *uuid.UUID, action string) error
}

type Repos struct {
	APIKey Repository
	Audit  AuditRepository
}

// Store is the Unit of Work Usecase depends on — same shape as every
// other domain's Store since ADR-011.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
