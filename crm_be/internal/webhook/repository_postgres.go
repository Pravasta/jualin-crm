package webhook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type postgresRepository struct {
	q db.Querier
}

// New returns the webhook_endpoints repository.
func New(q db.Querier) Repository {
	return &postgresRepository{q: q}
}

// NewDeliveryRepository returns the webhook_deliveries repository —
// separate constructor because its infrastructure methods (ClaimDue/
// Reap/Purge) are wired to the worker at a different point than the
// tenant-scoped endpoint CRUD.
func NewDeliveryRepository(q db.Querier) DeliveryRepository {
	return &postgresDeliveryRepository{q: q}
}

const endpointColumns = `id, organization_id, url, secret_ciphertext, secret_prefix, events, description, is_active, created_by_membership_id, created_at, updated_at, deleted_at`

func (r *postgresRepository) Create(ctx context.Context, t tenant.Context, e *Endpoint) error {
	const q = `
		INSERT INTO webhook_endpoints
			(id, organization_id, url, secret_ciphertext, secret_prefix, events, description, is_active, created_by_membership_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	err := r.q.QueryRow(ctx, q,
		e.ID, t.OrganizationID, e.URL, e.SecretCiphertext, e.SecretPrefix, e.Events, e.Description, e.IsActive, e.CreatedByMembershipID,
	).Scan(&e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("webhook: create endpoint: %w", err)
	}
	return nil
}

func (r *postgresRepository) FindByOrg(ctx context.Context, t tenant.Context) ([]*Endpoint, error) {
	const q = `
		SELECT ` + endpointColumns + `
		FROM webhook_endpoints
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.q.Query(ctx, q, t.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("webhook: find endpoints by org: %w", err)
	}
	defer rows.Close()

	var out []*Endpoint
	for rows.Next() {
		e, err := scanEndpoint(rows)
		if err != nil {
			return nil, fmt.Errorf("webhook: scan endpoint: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("webhook: find endpoints by org: %w", err)
	}
	return out, nil
}

// FindByID scopes to t.OrganizationID and excludes soft-deleted rows — a
// cross-org or already-deleted endpoint is indistinguishable from one
// that never existed (Rule #6).
func (r *postgresRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Endpoint, error) {
	const q = `
		SELECT ` + endpointColumns + `
		FROM webhook_endpoints
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`

	e, err := scanEndpoint(r.q.QueryRow(ctx, q, id, t.OrganizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("webhook: find endpoint by id: %w", err)
	}
	return e, nil
}

// Update applies the COALESCE pattern — same shape as
// form.postgresRepository.Update. Each parameter is passed as nil when
// the caller didn't touch it (UpdateInput's nil-means-unchanged
// convention), and the explicit ::type casts on each COALESCE are
// required because Postgres can't infer the type of a bare NULL
// parameter (the exact gap #85 hit).
func (r *postgresRepository) Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Endpoint, error) {
	const q = `
		UPDATE webhook_endpoints
		SET url         = COALESCE($3::text, url),
		    events      = COALESCE($4::text[], events),
		    description = COALESCE($5::text, description),
		    is_active   = COALESCE($6::boolean, is_active),
		    updated_at  = now()
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL
		RETURNING ` + endpointColumns

	var eventsArg any
	if in.Events != nil {
		eventsArg = *in.Events
	}
	e, err := scanEndpoint(r.q.QueryRow(ctx, q, id, t.OrganizationID,
		derefOrNil(in.URL), eventsArg, derefOrNil(in.Description), derefBoolOrNil(in.IsActive)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("webhook: update endpoint: %w", err)
	}
	return e, nil
}

// Delete is a soft delete. Usecase.Delete always calls FindByID first,
// so a missing/cross-org/already-deleted row 404s there — zero rows here
// would only mean a race with a concurrent delete.
func (r *postgresRepository) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	const q = `UPDATE webhook_endpoints SET deleted_at = now() WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`
	if _, err := r.q.Exec(ctx, q, id, t.OrganizationID); err != nil {
		return fmt.Errorf("webhook: delete endpoint: %w", err)
	}
	return nil
}

func derefOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func derefBoolOrNil(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEndpoint(row rowScanner) (*Endpoint, error) {
	var e Endpoint
	err := row.Scan(
		&e.ID, &e.OrganizationID, &e.URL, &e.SecretCiphertext, &e.SecretPrefix, &e.Events,
		&e.Description, &e.IsActive, &e.CreatedByMembershipID, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// --- webhook_deliveries ---

type postgresDeliveryRepository struct {
	q db.Querier
}

const deliveryColumns = `id, organization_id, endpoint_id, event_type, payload, status, attempt, next_attempt_at, response_status, error, delivering_since, created_at, updated_at`

// Enqueue inserts one pending delivery. Written INSIDE the triggering
// transaction by #101's Usecase.Enqueue — here in #100 it exists for
// repository_test.go to seed rows the tenant-scoped reads act on.
func (r *postgresDeliveryRepository) Enqueue(ctx context.Context, t tenant.Context, d *Delivery) error {
	const q = `
		INSERT INTO webhook_deliveries
			(id, organization_id, endpoint_id, event_type, payload)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING status, attempt, next_attempt_at, created_at, updated_at`

	err := r.q.QueryRow(ctx, q, d.ID, t.OrganizationID, d.EndpointID, d.EventType, d.Payload).
		Scan(&d.Status, &d.Attempt, &d.NextAttemptAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("webhook: enqueue delivery: %w", err)
	}
	d.OrganizationID = t.OrganizationID
	return nil
}

func (r *postgresDeliveryRepository) FindByEndpoint(ctx context.Context, t tenant.Context, endpointID uuid.UUID, page, perPage int) ([]*Delivery, int, error) {
	const q = `
		SELECT ` + deliveryColumns + `, count(*) OVER() AS total
		FROM webhook_deliveries
		WHERE organization_id = $1 AND endpoint_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.q.Query(ctx, q, t.OrganizationID, endpointID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("webhook: find deliveries by endpoint: %w", err)
	}
	defer rows.Close()

	var out []*Delivery
	var total int
	for rows.Next() {
		d, rowTotal, err := scanDeliveryWithTotal(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("webhook: scan delivery: %w", err)
		}
		total = rowTotal
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("webhook: find deliveries by endpoint: %w", err)
	}
	return out, total, nil
}

func (r *postgresDeliveryRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Delivery, error) {
	const q = `SELECT ` + deliveryColumns + ` FROM webhook_deliveries WHERE id = $1 AND organization_id = $2`
	d, err := scanDelivery(r.q.QueryRow(ctx, q, id, t.OrganizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("webhook: find delivery by id: %w", err)
	}
	return d, nil
}

// MarkForRetry resets a failed delivery to pending for an immediate
// attempt. WHERE status = 'failed' is the real guard — 0 rows affected
// means the row was missing, cross-org, or not failed; the usecase's
// FindByID pre-check has already ruled out the first two by the time
// this runs, so ErrDeliveryNotRetryable is the right answer for 0 rows.
func (r *postgresDeliveryRepository) MarkForRetry(ctx context.Context, t tenant.Context, id uuid.UUID) (*Delivery, error) {
	const q = `
		UPDATE webhook_deliveries
		SET status = 'pending', attempt = 0, next_attempt_at = now(),
		    response_status = NULL, error = NULL, delivering_since = NULL, updated_at = now()
		WHERE id = $1 AND organization_id = $2 AND status = 'failed'
		RETURNING ` + deliveryColumns

	d, err := scanDelivery(r.q.QueryRow(ctx, q, id, t.OrganizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeliveryNotRetryable
	}
	if err != nil {
		return nil, fmt.Errorf("webhook: mark for retry: %w", err)
	}
	return d, nil
}

// ClaimDue is the worker's claim query (TD §4.1) — not tenant-scoped
// (interface doc). FOR UPDATE SKIP LOCKED is what makes many workers safe
// without leader election: Postgres guarantees two transactions never
// lock the same row. Caller MUST run this inside a transaction; the
// returned rows are already status='delivering'.
func (r *postgresDeliveryRepository) ClaimDue(ctx context.Context, limit int) ([]*Delivery, error) {
	const q = `
		UPDATE webhook_deliveries
		SET status = 'delivering', delivering_since = now(), updated_at = now()
		WHERE id IN (
			SELECT id FROM webhook_deliveries
			WHERE status = 'pending' AND next_attempt_at <= now()
			ORDER BY next_attempt_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING ` + deliveryColumns

	rows, err := r.q.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("webhook: claim due: %w", err)
	}
	defer rows.Close()

	var out []*Delivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("webhook: scan claimed delivery: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("webhook: claim due: %w", err)
	}
	return out, nil
}

// Reap returns rows stuck in 'delivering' since before threshold to
// 'pending' (TD §4.2 — a worker crashed after claiming). Not
// tenant-scoped.
func (r *postgresDeliveryRepository) Reap(ctx context.Context, threshold time.Time) (int, error) {
	const q = `
		UPDATE webhook_deliveries
		SET status = 'pending', next_attempt_at = now(), delivering_since = NULL, updated_at = now()
		WHERE status = 'delivering' AND delivering_since < $1`

	tag, err := r.q.Exec(ctx, q, threshold)
	if err != nil {
		return 0, fmt.Errorf("webhook: reap: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// Purge deletes terminal rows older than before (TD §10 — retention).
// Never touches pending/delivering regardless of age. Not tenant-scoped.
func (r *postgresDeliveryRepository) Purge(ctx context.Context, before time.Time) (int, error) {
	const q = `
		DELETE FROM webhook_deliveries
		WHERE status IN ('succeeded', 'failed') AND created_at < $1`

	tag, err := r.q.Exec(ctx, q, before)
	if err != nil {
		return 0, fmt.Errorf("webhook: purge: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func scanDelivery(row rowScanner) (*Delivery, error) {
	var d Delivery
	err := row.Scan(
		&d.ID, &d.OrganizationID, &d.EndpointID, &d.EventType, &d.Payload, &d.Status, &d.Attempt,
		&d.NextAttemptAt, &d.ResponseStatus, &d.Error, &d.DeliveringSince, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func scanDeliveryWithTotal(row rowScanner) (*Delivery, int, error) {
	var d Delivery
	var total int
	err := row.Scan(
		&d.ID, &d.OrganizationID, &d.EndpointID, &d.EventType, &d.Payload, &d.Status, &d.Attempt,
		&d.NextAttemptAt, &d.ResponseStatus, &d.Error, &d.DeliveringSince, &d.CreatedAt, &d.UpdatedAt, &total,
	)
	if err != nil {
		return nil, 0, err
	}
	return &d, total, nil
}
