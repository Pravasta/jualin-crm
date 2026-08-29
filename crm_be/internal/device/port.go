package device

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is device_token's own table interface.
type Repository interface {
	// Upsert inserts tok, or — if tok.Token already identifies a row
	// (uq_device_tokens_token is global, not per-organization) — MOVES
	// that row to tok's organization_id/membership_id/platform and
	// bumps last_seen_at. This is the mechanism behind "a device that
	// changes hands gets re-registered, not duplicated" (migration
	// 0006's comment on uq_device_tokens_token).
	Upsert(ctx context.Context, t tenant.Context, tok *Token) error

	// FindByToken is tenant-scoped by organization_id — a token
	// registered under another organization is indistinguishable from
	// one that doesn't exist (Rule #6), same as every other tenant-
	// scoped FindByX in this codebase. Used by Usecase.Unregister to
	// confirm ownership before deleting (see its doc comment).
	FindByToken(ctx context.Context, t tenant.Context, token string) (*Token, error)

	// FindByMembership returns every device currently registered for
	// one membership — the fan-out list internal/lead's PushSender
	// bridge sends to. Not scoped to the CALLER's own membership: the
	// caller here is Usecase.PushToMembership, invoked with the
	// ASSIGNING actor's tenant.Context on behalf of the RECIPIENT
	// membership, which are frequently two different people.
	FindByMembership(ctx context.Context, t tenant.Context, membershipID uuid.UUID) ([]*Token, error)

	// DeleteByToken is tenant-scoped by organization_id only — NOT by
	// membership_id. It has two callers with different membership
	// contexts: Usecase.Unregister (which checks ownership itself
	// first, in Go, because the caller's t.MembershipID and the
	// token's owning membership must be compared as data, not baked
	// into the WHERE clause) and Usecase.PushToMembership's cleanup
	// path (which already knows the token belongs to the recipient
	// membership because it was just read via FindByMembership for
	// that exact membership — adding a second membership filter here
	// would just repeat a check already made).
	DeleteByToken(ctx context.Context, t tenant.Context, token string) error
}

// Repos bundles what a single Usecase call needs.
type Repos struct {
	DeviceToken Repository
}

// Store is the Unit of Work Usecase depends on — same shape as every
// other domain's since ADR-011. Every method here is a single
// statement, but the pattern is kept identical to every other domain
// rather than special-cased away, matching how apikey (issue #46) —
// also entirely single-statement operations — still exposes Store.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
