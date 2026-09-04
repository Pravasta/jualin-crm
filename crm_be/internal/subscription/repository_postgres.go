package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// postgresRepository is unexported — unlike the concrete types in
// packages with no local Repository interface (organization, user), this
// one must satisfy TWO differently-shaped consumer interfaces:
// auth.SubscriptionRepository (CreateFree only) via auth.Repos.Sub, and
// this package's own Repository (port.go, FindActiveByOrg only) via
// Usecase. Returning the concrete type from New — not either interface
// — is what lets both assignments type-check (Go checks a concrete
// type's full method set on assignment to any interface, but checking
// one interface against another only considers the source interface's
// declared methods).
type postgresRepository struct {
	q db.Querier
}

func New(q db.Querier) *postgresRepository {
	return &postgresRepository{q: q}
}

// CreateFree inserts the free-plan subscription every organization gets
// at registration. Plan/entitlement/usage enforcement is Phase 8 — this
// call has no side effect beyond the row existing.
func (r *postgresRepository) CreateFree(ctx context.Context, t tenant.Context, id uuid.UUID) (*Subscription, error) {
	const q = `
		INSERT INTO subscriptions (id, organization_id, plan_code, status)
		VALUES ($1, $2, 'free', 'active')
		RETURNING id, organization_id, plan_code, status, current_period_start, current_period_end, external_reference, created_at, updated_at`

	s, err := scanOne(r.q.QueryRow(ctx, q, id, t.OrganizationID))
	if err != nil {
		return nil, fmt.Errorf("subscription: create free: %w", err)
	}
	return s, nil
}

// FindActiveByOrg loads t.OrganizationID's currently-active subscription
// — the row uq_subscriptions_org_active guarantees is unique per
// organization. Returns ErrNoActiveSubscription (not the raw
// pgx.ErrNoRows) when there is none, so callers never need to import
// pgx to recognize this case (ADR-011).
func (r *postgresRepository) FindActiveByOrg(ctx context.Context, t tenant.Context) (*Subscription, error) {
	const q = `
		SELECT id, organization_id, plan_code, status, current_period_start, current_period_end, external_reference, created_at, updated_at
		FROM subscriptions
		WHERE organization_id = $1 AND status = 'active'`

	s, err := scanOne(r.q.QueryRow(ctx, q, t.OrganizationID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoActiveSubscription
		}
		return nil, fmt.Errorf("subscription: find active by org: %w", err)
	}
	return s, nil
}

func scanOne(row interface{ Scan(...any) error }) (*Subscription, error) {
	var s Subscription
	err := row.Scan(
		&s.ID, &s.OrganizationID, &s.PlanCode, &s.Status,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.ExternalReference,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
