package webhook

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is webhook's own webhook_endpoints table interface —
// tenant-scoped, TenantContext always second (skill jualin-backend).
type Repository interface {
	Create(ctx context.Context, t tenant.Context, e *Endpoint) error
	FindByOrg(ctx context.Context, t tenant.Context) ([]*Endpoint, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Endpoint, error)
	Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Endpoint, error)
	Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error
}

// DeliveryRepository covers webhook_deliveries. It splits into two
// halves with different scoping rules:
//
//   - Tenant-scoped (FindByEndpoint, FindByID, Retry) — read/act on one
//     organization's delivery history, always via t.OrganizationID.
//   - Infrastructure (ClaimDue, Reap, Purge) — the worker (#102) runs
//     these across ALL organizations. They deliberately take no
//     tenant.Context: which organization a claimed row belongs to is an
//     OUTPUT, not an input (TD §1.2, the same exception as
//     apikey.FindByKeyID one level over — there it's a constraint, here
//     it's the ix_webhook_deliveries_claim index).
//
// ClaimDue/Reap/Purge have no caller in #100 — they're exercised
// directly by repository_test.go (including an EXPLAIN proving ClaimDue
// is an index hit) and wired to the worker in #102, the exact
// form.FindByPublicKey-built-for-#87 precedent repeated.
type DeliveryRepository interface {
	Enqueue(ctx context.Context, t tenant.Context, d *Delivery) error
	FindByEndpoint(ctx context.Context, t tenant.Context, endpointID uuid.UUID, page, perPage int) ([]*Delivery, int, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Delivery, error)

	// MarkForRetry resets a failed delivery to pending for an immediate
	// manual retry (kriteria #11). Returns ErrDeliveryNotRetryable if the
	// row's status is not 'failed'.
	MarkForRetry(ctx context.Context, t tenant.Context, id uuid.UUID) (*Delivery, error)

	// ClaimDue atomically moves up to `limit` due 'pending' rows to
	// 'delivering' and returns them — FOR UPDATE SKIP LOCKED (TD §4.1).
	// Not tenant-scoped (see interface doc).
	ClaimDue(ctx context.Context, limit int) ([]*Delivery, error)

	// Reap returns 'delivering' rows stuck past `threshold` to 'pending'
	// (TD §4.2 — crash recovery). Not tenant-scoped.
	Reap(ctx context.Context, threshold time.Time) (int, error)

	// Purge deletes terminal rows (succeeded/failed) older than `before`
	// (TD §10 — retention). Never touches pending/delivering. Not
	// tenant-scoped.
	Purge(ctx context.Context, before time.Time) (int, error)
}

// AuditRepository is declared here (the consumer) per ADR-011 —
// *auditlog.Repository already satisfies it structurally.
type AuditRepository interface {
	Record(ctx context.Context, t tenant.Context, actorMembershipID *uuid.UUID, action string) error
}

type Repos struct {
	Endpoint Repository
	Delivery DeliveryRepository
	Audit    AuditRepository
}

// Store is the Unit of Work Usecase depends on — same shape as every
// other domain's Store since ADR-011.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}

// SecretCrypter is the consumer-declared interface for sealing signing
// secrets at rest — *crypter.Crypter satisfies it structurally. Declared
// here with primitives only, like URLValidator below, so this package
// never learns which cipher is behind it.
//
// Decrypt has no caller in #101: Create seals, and only the worker (#102)
// ever needs the plaintext back. It is declared now anyway because the
// pair is the interface — a Crypter that can only encrypt would be a
// write-only column, which is exactly the mistake migration 0009 fixes.
type SecretCrypter interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(sealed []byte) ([]byte, error)
}

// URLValidator is the consumer-declared interface for the SSRF guard —
// *safedial.Validator satisfies it structurally. Declared here with a
// primitive-only signature so internal/webhook never has to know how the
// deny-list works, only that a URL is safe to store.
type URLValidator interface {
	ValidateURL(ctx context.Context, rawURL string) error
}
