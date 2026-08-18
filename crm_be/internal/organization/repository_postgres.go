package organization

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

// Repository deliberately does NOT take tenant.Context — an organization
// IS the tenant, not something scoped to one. Looking one up by its own
// id needs no further scoping, the same way membership.Repository's
// FindByID scopes to organization_id but organizations themselves have
// no parent to scope against.
type Repository struct {
	q db.Querier
}

func New(q db.Querier) *Repository {
	return &Repository{q: q}
}

// Create inserts a new organization. Timezone defaults to Asia/Jakarta
// at the database level (migration 0002); callers that need a different
// timezone pass it explicitly via a future update path, not at creation.
func (r *Repository) Create(ctx context.Context, id uuid.UUID, name string) (*Organization, error) {
	const q = `
		INSERT INTO organizations (id, name)
		VALUES ($1, $2)
		RETURNING id, name, timezone, created_at, updated_at, deleted_at`

	o, err := scanOne(r.q.QueryRow(ctx, q, id, name))
	if err != nil {
		return nil, fmt.Errorf("organization: create: %w", err)
	}
	return o, nil
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	const q = `
		SELECT id, name, timezone, created_at, updated_at, deleted_at
		FROM organizations
		WHERE id = $1 AND deleted_at IS NULL`

	o, err := scanOne(r.q.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization: find by id: %w", err)
	}
	return o, nil
}

func scanOne(row pgx.Row) (*Organization, error) {
	var o Organization
	err := row.Scan(&o.ID, &o.Name, &o.Timezone, &o.CreatedAt, &o.UpdatedAt, &o.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
