package lead

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

const idempotencyUniqueConstraint = "uq_leads_org_idempotency"
const assigneeFKConstraint = "fk_leads_assignee"

// postgresRepository is the concrete implementation behind the
// Repository interface (port.go). Unexported since #20 — callers depend
// on the interface, not this type, so Usecase can be tested with a fake
// (ADR-011). Through #19 this was the exported concrete type
// (Repository); cross-package consumers never existed yet (there were
// none), so this rename is invisible outside the package.
type postgresRepository struct {
	q db.Querier
}

func New(q db.Querier) Repository {
	return &postgresRepository{q: q}
}

// NewOpenLeadRepository exposes the same concrete implementation as
// New, typed through OpenLeadRepository — see that interface's doc
// comment in port.go for why this exists (same pattern as
// auth.NewRefreshTokenRevoker).
func NewOpenLeadRepository(q db.Querier) OpenLeadRepository {
	return &postgresRepository{q: q}
}

// Create allocates the next lead_number for t.OrganizationID and inserts
// the row using the SAME r.q for both statements. The allocating
// UPDATE ... RETURNING is a single atomic statement, so concurrent
// callers never collide or duplicate a number regardless of whether r.q
// is a pool or a transaction (proven under real concurrency in
// repository_concurrency_test.go). What DOES depend on r.q being a
// pgx.Tx (via db.InTx) is what happens when the following INSERT
// fails: wrapped in a transaction, the whole thing rolls back —
// including the allocation — so a rejected Create doesn't burn a
// number; called against a bare pool, the allocation already committed
// on its own and the number is gone for good, a permanent gap. TD §3
// requires the transactional form. See
// TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber in
// repository_test.go for the proof of that specific guarantee.
func (r *postgresRepository) Create(ctx context.Context, t tenant.Context, in CreateInput) (*Lead, error) {
	const allocQ = `
		UPDATE organizations SET next_lead_number = next_lead_number + 1
		WHERE id = $1
		RETURNING next_lead_number - 1`

	var nextNumber int
	if err := r.q.QueryRow(ctx, allocQ, t.OrganizationID).Scan(&nextNumber); err != nil {
		return nil, fmt.Errorf("lead: allocate lead_number: %w", err)
	}

	const insertQ = `
		INSERT INTO leads (
			id, organization_id, lead_number, name, email, phone, phone_e164, company, notes,
			source, assigned_to_membership_id, raw_payload, idempotency_key, created_by_membership_id,
			source_api_key_id, source_form_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING ` + leadColumns

	id := uuid.Must(uuid.NewV7())
	row := r.q.QueryRow(ctx, insertQ,
		id, t.OrganizationID, nextNumber, in.Name, in.Email, in.Phone, in.PhoneE164, in.Company, in.Notes,
		in.Source, in.AssignedToMembershipID, in.RawPayload, in.IdempotencyKey, in.CreatedByMembershipID,
		in.SourceAPIKeyID, in.SourceFormID,
	)
	created, err := scanLead(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" && pgErr.ConstraintName == idempotencyUniqueConstraint {
				return nil, ErrIdempotencyKeyExists
			}
			// 23503 = FK violation. Found via a test that (by mistake)
			// assigned a lead to a membership id that didn't exist —
			// which surfaced as a bare 500 before this check existed.
			// assigned_to_membership_id is the only nullable FK a
			// caller-supplied value can violate here.
			if pgErr.Code == "23503" && pgErr.ConstraintName == assigneeFKConstraint {
				return nil, ErrAssigneeNotFound
			}
		}
		return nil, fmt.Errorf("lead: create: %w", err)
	}
	return created, nil
}

// FindByIdempotencyKey is how the usecase resolves ErrIdempotencyKeyExists
// into the existing lead to return instead of erroring.
func (r *postgresRepository) FindByIdempotencyKey(ctx context.Context, t tenant.Context, key string) (*Lead, error) {
	const q = `
		SELECT ` + leadColumns + `
		FROM leads
		WHERE organization_id = $1 AND idempotency_key = $2 AND deleted_at IS NULL`

	row := r.q.QueryRow(ctx, q, t.OrganizationID, key)
	found, err := scanLead(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lead: find by idempotency key: %w", err)
	}
	return found, nil
}

// FindByID is tenant-scoped (Rule #6: cross-org → httpx.ErrNotFound) and,
// when t.Role is employee, additionally scoped to leads assigned to
// t.MembershipID (TD §9 — the PRD's core requirement for this issue,
// enforced here once rather than in every future usecase method that
// touches a single lead).
func (r *postgresRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Lead, error) {
	const q = `
		SELECT ` + leadColumns + `
		FROM leads
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL
		  AND (NOT $3 OR assigned_to_membership_id = $4)`

	row := r.q.QueryRow(ctx, q, id, t.OrganizationID, isEmployee(t), membershipIDOrNil(t))
	found, err := scanLead(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lead: find by id: %w", err)
	}
	return found, nil
}

// Update applies optimistic locking (TD §4): expectedVersion must match
// the current row, and (like FindByID) an employee can only update a
// lead assigned to them. Nullable fields in in use "nil means don't
// touch" — this minimal Update has no way to clear a field to NULL;
// that's a deliberate simplification for issue #19's repository-only
// scope, revisited if #20's usecase needs it.
//
// Zero rows affected is ambiguous (wrong version vs. wrong tenant/scope
// vs. missing) — resolved by re-running FindByID's exact scoping query:
// row not visible in this scope → httpx.ErrNotFound; row visible but
// version differs → (current state, ErrVersionConflict), so the future
// usecase can build TD §4's 409 body without a second round trip.
func (r *postgresRepository) Update(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in UpdateInput) (*Lead, error) {
	const q = `
		UPDATE leads
		SET version = version + 1,
		    name    = COALESCE($5, name),
		    email   = COALESCE($6, email),
		    phone   = COALESCE($7, phone),
		    company = COALESCE($8, company),
		    notes   = COALESCE($9, notes)
		WHERE id = $1 AND organization_id = $2 AND version = $3 AND deleted_at IS NULL
		  AND (NOT $4 OR assigned_to_membership_id = $10)
		RETURNING ` + leadColumns

	row := r.q.QueryRow(ctx, q,
		id, t.OrganizationID, expectedVersion, isEmployee(t),
		in.Name, in.Email, in.Phone, in.Company, in.Notes,
		membershipIDOrNil(t),
	)
	updated, err := scanLead(row)
	if errors.Is(err, pgx.ErrNoRows) {
		current, findErr := r.FindByID(ctx, t, id)
		if findErr != nil {
			return nil, findErr
		}
		return current, ErrVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("lead: update: %w", err)
	}
	return updated, nil
}

// FindAllByOrg builds its WHERE clause by hand (no query-builder
// library — sqlc/pgx, not an ORM) from whichever filter fields are set,
// sharing one args slice between the count query and the paginated
// select so the two never drift apart. Employee scoping is applied
// UNCONDITIONALLY when t.Role is employee, regardless of filter.AssignedTo
// — an employee's own filter can only narrow their view further, never
// widen past their own leads (same rule FindByID/Update already enforce).
func (r *postgresRepository) FindAllByOrg(ctx context.Context, t tenant.Context, filter ListFilter) ([]*Lead, int, error) {
	conditions := []string{"organization_id = $1", "deleted_at IS NULL"}
	args := []any{t.OrganizationID}

	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if isEmployee(t) {
		conditions = append(conditions, "assigned_to_membership_id = "+arg(membershipIDOrNil(t)))
	} else if filter.AssignedToNone {
		conditions = append(conditions, "assigned_to_membership_id IS NULL")
	} else if filter.AssignedTo != nil {
		conditions = append(conditions, "assigned_to_membership_id = "+arg(*filter.AssignedTo))
	}

	if len(filter.Status) > 0 {
		conditions = append(conditions, "status = ANY("+arg(filter.Status)+")")
	}
	if len(filter.Source) > 0 {
		conditions = append(conditions, "source = ANY("+arg(filter.Source)+")")
	}
	if filter.Query != "" {
		p := arg("%" + filter.Query + "%")
		conditions = append(conditions, fmt.Sprintf("(name ILIKE %s OR email ILIKE %s OR phone_e164 ILIKE %s)", p, p, p))
	}
	if filter.CreatedFrom != nil {
		conditions = append(conditions, "created_at >= "+arg(*filter.CreatedFrom))
	}
	if filter.CreatedTo != nil {
		conditions = append(conditions, "created_at <= "+arg(*filter.CreatedTo))
	}

	where := strings.Join(conditions, " AND ")

	var total int
	countQ := "SELECT count(*) FROM leads WHERE " + where
	if err := r.q.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("lead: count: %w", err)
	}

	perPage := filter.PerPage
	offset := (filter.Page - 1) * perPage
	listQ := "SELECT " + leadColumns + " FROM leads WHERE " + where +
		" ORDER BY created_at DESC LIMIT " + arg(perPage) + " OFFSET " + arg(offset)

	rows, err := r.q.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("lead: find all by org: %w", err)
	}
	defer rows.Close()

	var out []*Lead
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("lead: scan: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("lead: find all by org: %w", err)
	}
	return out, total, nil
}

// UpdateStatus mirrors Update's version+scope-gated shape exactly, but
// only ever touches status/lost_reason/version — the transition
// validity check itself lives in Usecase.UpdateStatus (TD §5 is
// business logic, not a repository concern).
func (r *postgresRepository) UpdateStatus(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, status string, lostReason *string) (*Lead, error) {
	const q = `
		UPDATE leads
		SET version = version + 1, status = $5, lost_reason = $6
		WHERE id = $1 AND organization_id = $2 AND version = $3 AND deleted_at IS NULL
		  AND (NOT $4 OR assigned_to_membership_id = $7)
		RETURNING ` + leadColumns

	row := r.q.QueryRow(ctx, q,
		id, t.OrganizationID, expectedVersion, isEmployee(t),
		status, lostReason, membershipIDOrNil(t),
	)
	updated, err := scanLead(row)
	if errors.Is(err, pgx.ErrNoRows) {
		current, findErr := r.FindByID(ctx, t, id)
		if findErr != nil {
			return nil, findErr
		}
		return current, ErrVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("lead: update status: %w", err)
	}
	return updated, nil
}

// UpdateAssignment mirrors UpdateStatus's exact shape, touching only
// assigned_to_membership_id/version. A nil assignedTo clears it
// (unassign). The employee-scope guard is kept for consistency with
// every other mutating method here even though authz.ActionLeadAssign
// already blocks Employee from ever reaching this — same precedent
// Delete already set.
func (r *postgresRepository) UpdateAssignment(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, assignedTo *uuid.UUID) (*Lead, error) {
	const q = `
		UPDATE leads
		SET version = version + 1, assigned_to_membership_id = $5
		WHERE id = $1 AND organization_id = $2 AND version = $3 AND deleted_at IS NULL
		  AND (NOT $4 OR assigned_to_membership_id = $6)
		RETURNING ` + leadColumns

	row := r.q.QueryRow(ctx, q,
		id, t.OrganizationID, expectedVersion, isEmployee(t),
		assignedTo, membershipIDOrNil(t),
	)
	updated, err := scanLead(row)
	if errors.Is(err, pgx.ErrNoRows) {
		current, findErr := r.FindByID(ctx, t, id)
		if findErr != nil {
			return nil, findErr
		}
		return current, ErrVersionConflict
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == assigneeFKConstraint {
			return nil, ErrAssigneeNotFound
		}
		return nil, fmt.Errorf("lead: update assignment: %w", err)
	}
	return updated, nil
}

// Delete soft-deletes id within t's scope (Rule #18). No version check —
// TD doesn't gate delete on optimistic locking the way it does field/status
// updates; deleting an already-stale-looking row is still a safe delete.
func (r *postgresRepository) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	const q = `
		UPDATE leads SET deleted_at = now()
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL
		  AND (NOT $3 OR assigned_to_membership_id = $4)`

	tag, err := r.q.Exec(ctx, q, id, t.OrganizationID, isEmployee(t), membershipIDOrNil(t))
	if err != nil {
		return fmt.Errorf("lead: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

// openLeadCondition is TD §13's definition of "lead terbuka": status
// not in any of the four final states, and not soft-deleted (the
// deleted_at check lives in each caller alongside this constant since
// it's ANDed with other column references there).
const openLeadCondition = `status NOT IN ('won','lost','unqualified','spam')`

// CountOpen, UnassignOpen, and ReassignOpen back OpenLeadRepository —
// membership.Usecase.Deactivate's only way to ask "does this membership
// still own open leads" and act on the answer, without membership
// importing this package (see OpenLeadRepository's doc comment in
// port.go). None of these take an employee-scope guard: they operate on
// ALL leads assigned to a specific membershipID regardless of who's
// calling — the caller (membership) already gated this to Owner/Admin
// via its own authz check before ever reaching here.

func (r *postgresRepository) CountOpen(ctx context.Context, t tenant.Context, membershipID uuid.UUID) (int, error) {
	const q = `
		SELECT count(*) FROM leads
		WHERE organization_id = $1 AND assigned_to_membership_id = $2 AND deleted_at IS NULL
		  AND ` + openLeadCondition

	var count int
	if err := r.q.QueryRow(ctx, q, t.OrganizationID, membershipID).Scan(&count); err != nil {
		return 0, fmt.Errorf("lead: count open: %w", err)
	}
	return count, nil
}

// UnassignOpen clears assigned_to_membership_id for every open lead
// assigned to membershipID, returning their ids so the caller can log
// one lead_unassigned activity per lead in the same transaction (TD
// §13).
func (r *postgresRepository) UnassignOpen(ctx context.Context, t tenant.Context, membershipID uuid.UUID) ([]uuid.UUID, error) {
	const q = `
		UPDATE leads SET version = version + 1, assigned_to_membership_id = NULL
		WHERE organization_id = $1 AND assigned_to_membership_id = $2 AND deleted_at IS NULL
		  AND ` + openLeadCondition + `
		RETURNING id`

	return scanLeadIDs(r.q.Query(ctx, q, t.OrganizationID, membershipID))
}

// ReassignOpen moves every open lead assigned to membershipID to
// reassignTo, returning their ids for the same reason UnassignOpen
// does. A reassignTo that isn't a real membership in this organization
// violates fk_leads_assignee — mapped directly to *httpx.ValidationError
// here rather than the ErrAssigneeNotFound sentinel Create/
// UpdateAssignment use: those sentinels are caught and translated by
// lead's OWN usecase, but this method is only ever called by
// membership.Usecase.Deactivate through the OpenLeadRepository bridge,
// which can't import lead to recognize a lead-specific sentinel
// (ADR-011). httpx is the one neutral package both sides already
// import, and httpx.MapError already knows how to turn a
// *httpx.ValidationError into 400 with no extra code in membership.
func (r *postgresRepository) ReassignOpen(ctx context.Context, t tenant.Context, membershipID, reassignTo uuid.UUID) ([]uuid.UUID, error) {
	const q = `
		UPDATE leads SET version = version + 1, assigned_to_membership_id = $3
		WHERE organization_id = $1 AND assigned_to_membership_id = $2 AND deleted_at IS NULL
		  AND ` + openLeadCondition + `
		RETURNING id`

	ids, err := scanLeadIDs(r.q.Query(ctx, q, t.OrganizationID, membershipID, reassignTo))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == assigneeFKConstraint {
			return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "reassign_to", Code: "not_found"})
		}
		return nil, err
	}
	return ids, nil
}

func scanLeadIDs(rows pgx.Rows, queryErr error) ([]uuid.UUID, error) {
	if queryErr != nil {
		return nil, fmt.Errorf("lead: query open leads: %w", queryErr)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("lead: scan open lead id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lead: query open leads: %w", err)
	}
	return ids, nil
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

const leadColumns = `
	id, organization_id, lead_number, name, email, phone, phone_e164, company, notes,
	status, lost_reason, source, assigned_to_membership_id, raw_payload, idempotency_key,
	version, created_by_membership_id, created_at, updated_at, deleted_at, source_api_key_id,
	source_form_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLead(row rowScanner) (*Lead, error) {
	var l Lead
	err := row.Scan(
		&l.ID, &l.OrganizationID, &l.LeadNumber, &l.Name, &l.Email, &l.Phone, &l.PhoneE164, &l.Company, &l.Notes,
		&l.Status, &l.LostReason, &l.Source, &l.AssignedToMembershipID, &l.RawPayload, &l.IdempotencyKey,
		&l.Version, &l.CreatedByMembershipID, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt, &l.SourceAPIKeyID,
		&l.SourceFormID,
	)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// CleanupExpiredIdempotencyKeys clears idempotency_key on leads (not the
// leads themselves) older than 48h — see the interface doc comment in
// port.go for the full rationale (TD §7, keputusan D3).
func (r *postgresRepository) CleanupExpiredIdempotencyKeys(ctx context.Context, t tenant.Context) error {
	const q = `
		UPDATE leads
		   SET idempotency_key = NULL
		 WHERE organization_id = $1
		   AND idempotency_key IS NOT NULL
		   AND created_at < now() - interval '48 hours'`

	if _, err := r.q.Exec(ctx, q, t.OrganizationID); err != nil {
		return fmt.Errorf("lead: cleanup expired idempotency keys: %w", err)
	}
	return nil
}
