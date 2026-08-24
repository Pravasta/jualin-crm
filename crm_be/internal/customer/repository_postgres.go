package customer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const alreadyConvertedConstraint = "uq_customers_org_lead"

type postgresRepository struct {
	q db.Querier
}

func New(q db.Querier) Repository {
	return &postgresRepository{q: q}
}

// Convert copies leadID's contact fields into a new customer row in one
// statement — INSERT ... SELECT FROM leads, scoped to status='won' —
// rather than reading the lead in Go first and passing its fields back
// down: this package has direct db.Querier access to the SAME database
// leads lives in, so there's no need for a Go-level bridge into
// internal/lead (the trick activity/task already use for lead
// visibility checks).
//
// Zero rows is ambiguous between "lead not visible" and "lead visible
// but not won" — resolved by a follow-up existence-only check, same
// disambiguation shape as every optimistic-locking zero-row case
// elsewhere in this codebase. A unique violation on
// uq_customers_org_lead means this lead was already converted.
func (r *postgresRepository) Convert(ctx context.Context, t tenant.Context, leadID uuid.UUID, convertedByMembershipID *uuid.UUID) (*Customer, error) {
	const q = `
		INSERT INTO customers (
			id, organization_id, name, email, phone, phone_e164, company, notes,
			converted_from_lead_id, converted_by_membership_id
		)
		SELECT $1, $2, name, email, phone, phone_e164, company, notes, id, $3
		FROM leads
		WHERE id = $4 AND organization_id = $2 AND deleted_at IS NULL AND status = 'won'
		RETURNING ` + customerColumns

	id := uuid.Must(uuid.NewV7())
	row := r.q.QueryRow(ctx, q, id, t.OrganizationID, convertedByMembershipID, leadID)
	created, err := scanCustomer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		visible, visErr := r.leadVisible(ctx, t, leadID)
		if visErr != nil {
			return nil, visErr
		}
		if !visible {
			return nil, httpx.ErrNotFound
		}
		return nil, ErrLeadNotWon
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == alreadyConvertedConstraint {
			return nil, ErrAlreadyConverted
		}
		return nil, fmt.Errorf("customer: convert: %w", err)
	}
	return created, nil
}

func (r *postgresRepository) leadVisible(ctx context.Context, t tenant.Context, leadID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM leads WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL)`
	var visible bool
	if err := r.q.QueryRow(ctx, q, leadID, t.OrganizationID).Scan(&visible); err != nil {
		return false, fmt.Errorf("customer: check lead visibility: %w", err)
	}
	return visible, nil
}

// FindByID scopes Employee visibility through the ORIGINATING lead's
// assignment (TD §9 footnote: "customer dari lead itu"), not any field
// on customers itself — customers has no assigned_to_membership_id.
func (r *postgresRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Customer, error) {
	const q = `
		SELECT ` + customerColumns + `
		FROM customers c
		WHERE c.id = $1 AND c.organization_id = $2 AND c.deleted_at IS NULL
		  AND (NOT $3 OR EXISTS (
		      SELECT 1 FROM leads l WHERE l.id = c.converted_from_lead_id AND l.organization_id = $2
		        AND l.assigned_to_membership_id = $4
		  ))`

	row := r.q.QueryRow(ctx, q, id, t.OrganizationID, isEmployee(t), membershipIDOrNil(t))
	found, err := scanCustomer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("customer: find by id: %w", err)
	}
	return found, nil
}

func (r *postgresRepository) FindAllByOrg(ctx context.Context, t tenant.Context, filter ListFilter) ([]*Customer, int, error) {
	conditions := []string{"c.organization_id = $1", "c.deleted_at IS NULL"}
	args := []any{t.OrganizationID}

	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if isEmployee(t) {
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM leads l WHERE l.id = c.converted_from_lead_id AND l.organization_id = $1 AND l.assigned_to_membership_id = %s)",
			arg(membershipIDOrNil(t))))
	}
	if filter.Query != "" {
		p := arg("%" + filter.Query + "%")
		conditions = append(conditions, fmt.Sprintf("(c.name ILIKE %s OR c.email ILIKE %s OR c.phone_e164 ILIKE %s)", p, p, p))
	}

	where := strings.Join(conditions, " AND ")

	var total int
	countQ := "SELECT count(*) FROM customers c WHERE " + where
	if err := r.q.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("customer: count: %w", err)
	}

	perPage := filter.PerPage
	offset := (filter.Page - 1) * perPage
	listQ := "SELECT " + customerColumns + " FROM customers c WHERE " + where +
		" ORDER BY c.created_at DESC LIMIT " + arg(perPage) + " OFFSET " + arg(offset)

	rows, err := r.q.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("customer: find all by org: %w", err)
	}
	defer rows.Close()

	var out []*Customer
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("customer: scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("customer: find all by org: %w", err)
	}
	return out, total, nil
}

// Update touches only general fields — no version column exists on
// customers (TD §8: not edited from offline mobile, so no write
// conflict to detect), so zero rows affected is a plain 404, not a
// conflict to disambiguate.
func (r *postgresRepository) Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Customer, error) {
	const q = `
		UPDATE customers c
		SET name    = COALESCE($4, name),
		    email   = COALESCE($5, email),
		    phone   = COALESCE($6, phone),
		    company = COALESCE($7, company),
		    notes   = COALESCE($8, notes)
		WHERE c.id = $1 AND c.organization_id = $2 AND c.deleted_at IS NULL
		  AND (NOT $3 OR EXISTS (
		      SELECT 1 FROM leads l WHERE l.id = c.converted_from_lead_id AND l.organization_id = $2
		        AND l.assigned_to_membership_id = $9
		  ))
		RETURNING ` + customerColumns

	row := r.q.QueryRow(ctx, q,
		id, t.OrganizationID, isEmployee(t),
		in.Name, in.Email, in.Phone, in.Company, in.Notes,
		membershipIDOrNil(t),
	)
	updated, err := scanCustomer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("customer: update: %w", err)
	}
	return updated, nil
}

func (r *postgresRepository) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	const q = `
		UPDATE customers c SET deleted_at = now()
		WHERE c.id = $1 AND c.organization_id = $2 AND c.deleted_at IS NULL
		  AND (NOT $3 OR EXISTS (
		      SELECT 1 FROM leads l WHERE l.id = c.converted_from_lead_id AND l.organization_id = $2
		        AND l.assigned_to_membership_id = $4
		  ))`

	tag, err := r.q.Exec(ctx, q, id, t.OrganizationID, isEmployee(t), membershipIDOrNil(t))
	if err != nil {
		return fmt.Errorf("customer: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

func isEmployee(t tenant.Context) bool {
	return t.Role == tenant.RoleEmployee
}

func membershipIDOrNil(t tenant.Context) uuid.UUID {
	if t.MembershipID == nil {
		return uuid.UUID{}
	}
	return *t.MembershipID
}

// customerColumns is deliberately UNALIASED — Convert's RETURNING
// clause has no "c" alias in scope (INSERT never aliases its target
// table), while the SELECT/UPDATE/DELETE queries below alias customers
// as "c" only for their WHERE/EXISTS clauses; Postgres resolves these
// bare column names against the sole customers reference in either
// context without ambiguity.
const customerColumns = `
	id, organization_id, name, email, phone, phone_e164, company, notes,
	converted_from_lead_id, converted_by_membership_id, converted_at,
	created_at, updated_at, deleted_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCustomer(row rowScanner) (*Customer, error) {
	var c Customer
	err := row.Scan(
		&c.ID, &c.OrganizationID, &c.Name, &c.Email, &c.Phone, &c.PhoneE164, &c.Company, &c.Notes,
		&c.ConvertedFromLeadID, &c.ConvertedByMembershipID, &c.ConvertedAt,
		&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
