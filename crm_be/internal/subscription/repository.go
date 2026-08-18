package subscription

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type Repository struct {
	q db.Querier
}

func New(q db.Querier) *Repository {
	return &Repository{q: q}
}

// CreateFree inserts the free-plan subscription every organization gets
// at registration. Plan/entitlement/usage enforcement is Phase 8 — this
// call has no side effect beyond the row existing.
func (r *Repository) CreateFree(ctx context.Context, t tenant.Context, id uuid.UUID) (*Subscription, error) {
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
