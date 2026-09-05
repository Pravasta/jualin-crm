package subscription

import (
	"context"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is subscription's own interface — *postgresRepository
// (repository_postgres.go) satisfies it, but Usecase depends only on
// this interface (ADR-011).
//
// No Store here despite ChangePlan being a write (Phase 8.5 #124):
// every method here is a single-table, single-row statement, never
// needing atomicity with a second repository the way Store.InTx exists
// for. CreateFree — the one write this domain had before #124 — is
// still called as a plain repository method from inside internal/auth's
// registration transaction (auth.Repos.Sub), same precedent as
// internal/metrics (#30).
type Repository interface {
	FindActiveByOrg(ctx context.Context, t tenant.Context) (*Subscription, error)

	// ChangePlan sets t.OrganizationID's subscription row to planCode
	// (Phase 8.5 #124) — called only from the two admin surfaces in
	// cmd/api (token-gated internal endpoint, and the Owner-triggered
	// test-checkout), never from a normal user-facing write path. Does
	// NOT touch status: this issue changes what plan an organization is
	// on, not whether its billing is current — those are orthogonal
	// (TD §1.1).
	ChangePlan(ctx context.Context, t tenant.Context, planCode string) error
}
