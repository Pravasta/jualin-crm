package lead

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

// Repository is a plain concrete type — no interface — mirroring
// internal/membership's shape before #11 gave it a real usecase.
// Interface-at-consumer (ADR-011) only makes sense once a consumer
// exists to declare it; #20 adds port.go + usecase.go and renames this
// to postgresRepository at that point, same migration membership already
// went through.
type Repository struct {
	q db.Querier
}

func New(q db.Querier) *Repository {
	return &Repository{q: q}
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
func (r *Repository) Create(ctx context.Context, t tenant.Context, in CreateInput) (*Lead, error) {
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
			source, assigned_to_membership_id, raw_payload, idempotency_key, created_by_membership_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING ` + leadColumns

	id := uuid.Must(uuid.NewV7())
	row := r.q.QueryRow(ctx, insertQ,
		id, t.OrganizationID, nextNumber, in.Name, in.Email, in.Phone, in.PhoneE164, in.Company, in.Notes,
		in.Source, in.AssignedToMembershipID, in.RawPayload, in.IdempotencyKey, in.CreatedByMembershipID,
	)
	created, err := scanLead(row)
	if err != nil {
		return nil, fmt.Errorf("lead: create: %w", err)
	}
	return created, nil
}

// FindByID is tenant-scoped (Rule #6: cross-org → httpx.ErrNotFound) and,
// when t.Role is employee, additionally scoped to leads assigned to
// t.MembershipID (TD §9 — the PRD's core requirement for this issue,
// enforced here once rather than in every future usecase method that
// touches a single lead).
func (r *Repository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Lead, error) {
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
func (r *Repository) Update(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in UpdateInput) (*Lead, error) {
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
	version, created_by_membership_id, created_at, updated_at, deleted_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLead(row rowScanner) (*Lead, error) {
	var l Lead
	err := row.Scan(
		&l.ID, &l.OrganizationID, &l.LeadNumber, &l.Name, &l.Email, &l.Phone, &l.PhoneE164, &l.Company, &l.Notes,
		&l.Status, &l.LostReason, &l.Source, &l.AssignedToMembershipID, &l.RawPayload, &l.IdempotencyKey,
		&l.Version, &l.CreatedByMembershipID, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &l, nil
}
