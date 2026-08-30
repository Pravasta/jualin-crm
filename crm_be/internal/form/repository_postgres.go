package form

import (
	"context"
	"encoding/json"
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

// formColumns is deliberately unaliased — every query below references
// forms as its only table, so bare column names never need
// disambiguating (same reasoning as customer.customerColumns).
const formColumns = `id, organization_id, public_key, name, fields, allowed_origins, submit_count, created_by_membership_id, created_at, updated_at, deleted_at`

func (r *postgresRepository) Create(ctx context.Context, t tenant.Context, f *Form) error {
	fieldsJSON, err := json.Marshal(f.Fields)
	if err != nil {
		return fmt.Errorf("form: marshal fields: %w", err)
	}

	const q = `
		INSERT INTO forms (id, organization_id, public_key, name, fields, allowed_origins, created_by_membership_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`

	err = r.q.QueryRow(ctx, q,
		f.ID, t.OrganizationID, f.PublicKey, f.Name, fieldsJSON, f.AllowedOrigins, f.CreatedByMembershipID,
	).Scan(&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return fmt.Errorf("form: create: %w", err)
	}
	return nil
}

// FindByOrg excludes soft-deleted rows — TD §1's deleted_at, unlike
// api_keys' revoked_at, actually removes a form from this list (a
// deactivated form is meant to disappear from view, not stay visible
// with a strikethrough the way a revoked API key does).
func (r *postgresRepository) FindByOrg(ctx context.Context, t tenant.Context) ([]*Form, error) {
	const q = `
		SELECT ` + formColumns + `
		FROM forms
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.q.Query(ctx, q, t.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("form: find by org: %w", err)
	}
	defer rows.Close()

	var out []*Form
	for rows.Next() {
		f, err := scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("form: scan: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("form: find by org: %w", err)
	}
	return out, nil
}

// FindByID scopes to t.OrganizationID AND excludes soft-deleted rows —
// a form belonging to another organization, or one this organization
// already deleted, is indistinguishable from one that never existed
// (Rule #6).
func (r *postgresRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Form, error) {
	const q = `
		SELECT ` + formColumns + `
		FROM forms
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`

	f, err := scanRow(r.q.QueryRow(ctx, q, id, t.OrganizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("form: find by id: %w", err)
	}
	return f, nil
}

// Update applies UpdateInput's COALESCE pattern — same shape as
// customer.postgresRepository.Update. fields/allowed_origins are passed
// as nil when the caller didn't touch them so COALESCE leaves the
// stored value untouched, not overwritten with an empty one.
func (r *postgresRepository) Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Form, error) {
	var fieldsJSON []byte
	if in.Fields != nil {
		marshaled, err := json.Marshal(*in.Fields)
		if err != nil {
			return nil, fmt.Errorf("form: marshal fields: %w", err)
		}
		fieldsJSON = marshaled
	}
	const q = `
		UPDATE forms
		SET name             = COALESCE($3::text, name),
		    fields           = COALESCE($4::jsonb, fields),
		    allowed_origins  = COALESCE($5::text[], allowed_origins),
		    updated_at       = now()
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL
		RETURNING ` + formColumns

	row := r.q.QueryRow(ctx, q, id, t.OrganizationID, in.Name, nullIfEmpty(fieldsJSON), allowedOriginsOrNil(in))
	f, err := scanRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("form: update: %w", err)
	}
	return f, nil
}

// nullIfEmpty lets a nil *Fields (§Update didn't touch it) reach
// COALESCE as a real SQL NULL rather than an empty (but non-nil) byte
// slice, which pgx would otherwise bind as an empty string — not the
// same thing as "leave the column alone".
func nullIfEmpty(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// allowedOriginsOrNil mirrors nullIfEmpty for the text[] column —
// in.AllowedOrigins == nil means "don't touch it" (UpdateInput's own
// doc comment); a non-nil-but-empty slice means "clear it", and pgx
// binds that as an empty array, not NULL, which is exactly right.
func allowedOriginsOrNil(in UpdateInput) any {
	if in.AllowedOrigins == nil {
		return nil
	}
	return *in.AllowedOrigins
}

// Delete never returns httpx.ErrNotFound itself — Usecase.Delete always
// calls FindByID first (which is where the 404 for missing/cross-org/
// already-deleted comes from, see its doc comment), so by the time this
// runs the row is known to exist and not yet deleted; zero rows
// affected here would only mean a race with a concurrent delete, which
// is not worth surfacing as an error.
func (r *postgresRepository) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	const q = `UPDATE forms SET deleted_at = now() WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`
	if _, err := r.q.Exec(ctx, q, id, t.OrganizationID); err != nil {
		return fmt.Errorf("form: delete: %w", err)
	}
	return nil
}

// FindByPublicKey is NOT organization-scoped — see the exception
// documented on the Repository interface in port.go. WHERE public_key
// = $1 hits uq_forms_public_key's index directly (verified via EXPLAIN
// in repository_test.go) — this is the lookup #87's ResolvePublicKey
// will call. Soft-deleted forms are excluded: a deleted form's key must
// resolve exactly like one that never existed (Rule #6's same logic
// applied to a credential instead of a resource).
func (r *postgresRepository) FindByPublicKey(ctx context.Context, publicKey string) (*Form, error) {
	const q = `
		SELECT ` + formColumns + `
		FROM forms
		WHERE public_key = $1 AND deleted_at IS NULL`

	f, err := scanRow(r.q.QueryRow(ctx, q, publicKey))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("form: find by public key: %w", err)
	}
	return f, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(row rowScanner) (*Form, error) {
	var f Form
	var fieldsJSON []byte
	err := row.Scan(
		&f.ID, &f.OrganizationID, &f.PublicKey, &f.Name, &fieldsJSON, &f.AllowedOrigins, &f.SubmitCount,
		&f.CreatedByMembershipID, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fieldsJSON, &f.Fields); err != nil {
		return nil, fmt.Errorf("form: unmarshal fields: %w", err)
	}
	return &f, nil
}
