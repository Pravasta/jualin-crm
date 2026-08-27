package apikey

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type postgresRepository struct {
	q db.Querier
}

func New(q db.Querier) Repository {
	return &postgresRepository{q: q}
}

// apiKeyColumns is deliberately unaliased — every query below references
// api_keys as its only table, so bare column names never need
// disambiguating (same reasoning as customer.customerColumns).
const apiKeyColumns = `
	id, organization_id, key_id, secret_hash, key_prefix, name, scopes,
	created_by_membership_id, created_at, last_used_at, revoked_at, expires_at`

func (r *postgresRepository) Create(ctx context.Context, t tenant.Context, k *APIKey) error {
	const q = `
		INSERT INTO api_keys (id, organization_id, key_id, secret_hash, key_prefix, name, scopes, created_by_membership_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at`

	err := r.q.QueryRow(ctx, q,
		k.ID, t.OrganizationID, k.KeyID, k.SecretHash, k.KeyPrefix, k.Name, k.Scopes, k.CreatedByMembershipID,
	).Scan(&k.CreatedAt)
	if err != nil {
		return fmt.Errorf("apikey: create: %w", err)
	}
	return nil
}

// FindByOrg lists every credential for t.OrganizationID, revoked ones
// included — TD §1.1: revoked keys stay visible, they're never deleted.
func (r *postgresRepository) FindByOrg(ctx context.Context, t tenant.Context) ([]*APIKey, error) {
	const q = `
		SELECT ` + apiKeyColumns + `
		FROM api_keys
		WHERE organization_id = $1
		ORDER BY created_at DESC`

	rows, err := r.q.Query(ctx, q, t.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("apikey: find by org: %w", err)
	}
	defer rows.Close()

	var out []*APIKey
	for rows.Next() {
		k, err := scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("apikey: scan: %w", err)
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("apikey: find by org: %w", err)
	}
	return out, nil
}

// FindByID scopes to t.OrganizationID — a key belonging to another
// organization is indistinguishable from one that doesn't exist (Rule
// #6), same as every other tenant-scoped FindByID in this codebase.
func (r *postgresRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*APIKey, error) {
	const q = `
		SELECT ` + apiKeyColumns + `
		FROM api_keys
		WHERE id = $1 AND organization_id = $2`

	k, err := scanRow(r.q.QueryRow(ctx, q, id, t.OrganizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("apikey: find by id: %w", err)
	}
	return k, nil
}

// Revoke is idempotent: zero rows affected (key already revoked, or
// gone) is NOT an error here — Usecase.Revoke already confirmed
// existence via FindByID before calling this, so the only remaining
// possibility is "already revoked", which TD §9 requires to still
// answer 204.
func (r *postgresRepository) Revoke(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	const q = `UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND organization_id = $2 AND revoked_at IS NULL`
	if _, err := r.q.Exec(ctx, q, id, t.OrganizationID); err != nil {
		return fmt.Errorf("apikey: revoke: %w", err)
	}
	return nil
}

// FindByKeyID is NOT organization-scoped — see the exception documented
// on the Repository interface in port.go. WHERE key_id = $1 hits
// uq_api_keys_key_id's index directly (verified via EXPLAIN in
// repository_test.go) — this is the lookup ADR-004's verification steps
// describe, built now for #47 to call.
func (r *postgresRepository) FindByKeyID(ctx context.Context, keyID string) (*APIKey, error) {
	const q = `
		SELECT ` + apiKeyColumns + `
		FROM api_keys
		WHERE key_id = $1`

	k, err := scanRow(r.q.QueryRow(ctx, q, keyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("apikey: find by key id: %w", err)
	}
	return k, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(row rowScanner) (*APIKey, error) {
	var k APIKey
	err := row.Scan(
		&k.ID, &k.OrganizationID, &k.KeyID, &k.SecretHash, &k.KeyPrefix, &k.Name, &k.Scopes,
		&k.CreatedByMembershipID, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt, &k.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}
