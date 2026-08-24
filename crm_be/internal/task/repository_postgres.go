package task

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

const assigneeFKConstraint = "fk_tasks_assignee"

type postgresRepository struct {
	q db.Querier
}

func New(q db.Querier) Repository {
	return &postgresRepository{q: q}
}

// Create inserts a task, but only if leadID is visible to t — same
// tenant+employee visibility rule as activity.Create, expressed the
// same way (WHERE EXISTS against leads, no cross-package call). Zero
// rows (lead not visible) → httpx.ErrNotFound. A bad
// assigned_to_membership_id violates fk_tasks_assignee (23503),
// detected the same way lead.Create detects fk_leads_assignee since
// #20 — proactive, not speculative: it's the identical shape of bug
// already found once.
func (r *postgresRepository) Create(ctx context.Context, t tenant.Context, in CreateInput) (*Task, error) {
	const q = `
		INSERT INTO tasks (
			id, organization_id, lead_id, title, description, due_at,
			assigned_to_membership_id, created_by_membership_id
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8
		WHERE EXISTS (
			SELECT 1 FROM leads
			WHERE id = $3 AND organization_id = $2 AND deleted_at IS NULL
			  AND (NOT $9 OR assigned_to_membership_id = $10)
		)
		RETURNING ` + taskColumns

	id := uuid.Must(uuid.NewV7())
	row := r.q.QueryRow(ctx, q,
		id, t.OrganizationID, in.LeadID, in.Title, in.Description, in.DueAt,
		in.AssignedToMembershipID, in.CreatedByMembershipID,
		isEmployee(t), membershipIDOrNil(t),
	)
	created, err := scanTask(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == assigneeFKConstraint {
			return nil, ErrAssigneeNotFound
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.ErrNotFound
		}
		return nil, fmt.Errorf("task: create: %w", err)
	}
	return created, nil
}

// FindByID is tenant-scoped (Rule #6) and, for an employee, further
// scoped through the LEAD's assignment, not the task's own — TD §9:
// "task pada lead miliknya", not "task yang di-assign ke saya". A task
// can be assigned to a colleague on a lead you own and you must still
// see it; a task on someone else's lead must stay invisible even if
// it's assigned to you.
func (r *postgresRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Task, error) {
	const q = `
		SELECT ` + taskColumns + `
		FROM tasks
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL
		  AND (NOT $3 OR EXISTS (
		      SELECT 1 FROM leads l WHERE l.id = tasks.lead_id AND l.organization_id = $2
		        AND l.assigned_to_membership_id = $4
		  ))`

	row := r.q.QueryRow(ctx, q, id, t.OrganizationID, isEmployee(t), membershipIDOrNil(t))
	found, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("task: find by id: %w", err)
	}
	return found, nil
}

// FindAllByOrg is GET /v1/tasks's backing query — org-wide, paginated.
func (r *postgresRepository) FindAllByOrg(ctx context.Context, t tenant.Context, filter ListFilter) ([]*Task, int, error) {
	where, args := buildTaskWhere(t, filter)

	var total int
	countQ := "SELECT count(*) FROM tasks WHERE " + where
	if err := r.q.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("task: count: %w", err)
	}

	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	perPage := filter.PerPage
	offset := (filter.Page - 1) * perPage
	listQ := "SELECT " + taskColumns + " FROM tasks WHERE " + where +
		" ORDER BY created_at DESC LIMIT " + arg(perPage) + " OFFSET " + arg(offset)

	rows, err := r.q.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("task: find all by org: %w", err)
	}
	defer rows.Close()

	out, err := scanTasks(rows)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// FindAllByLead is GET /v1/leads/{id}/tasks's backing query — a single
// lead's tasks, unpaginated (TD §8's endpoint table has no page/per_page
// for this route). Unlike FindAllByOrg, this checks lead visibility
// EXPLICITLY and returns httpx.ErrNotFound when it fails, rather than
// letting employee-scoping quietly filter the list down to empty — the
// acceptance criteria are explicit that reading a sub-resource of a
// lead you can't see is a 404, same as activity.FindAllByLead.
func (r *postgresRepository) FindAllByLead(ctx context.Context, t tenant.Context, leadID uuid.UUID) ([]*Task, error) {
	visible, err := r.leadVisible(ctx, t, leadID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, httpx.ErrNotFound
	}

	const q = `
		SELECT ` + taskColumns + `
		FROM tasks
		WHERE organization_id = $1 AND lead_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC`
	rows, err := r.q.Query(ctx, q, t.OrganizationID, leadID)
	if err != nil {
		return nil, fmt.Errorf("task: find all by lead: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

func (r *postgresRepository) leadVisible(ctx context.Context, t tenant.Context, leadID uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM leads
			WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL
			  AND (NOT $3 OR assigned_to_membership_id = $4)
		)`
	var visible bool
	if err := r.q.QueryRow(ctx, q, leadID, t.OrganizationID, isEmployee(t), membershipIDOrNil(t)).Scan(&visible); err != nil {
		return false, fmt.Errorf("task: check lead visibility: %w", err)
	}
	return visible, nil
}

// buildTaskWhere backs FindAllByOrg — employee scoping here quietly
// narrows an org-wide list, which is correct for that endpoint (same as
// lead.FindAllByOrg). FindAllByLead does NOT use this: a single lead's
// task list needs an explicit visible/not-visible answer (404), not a
// filter that happens to return zero rows.
func buildTaskWhere(t tenant.Context, filter ListFilter) (string, []any) {
	conditions := []string{"organization_id = $1", "deleted_at IS NULL"}
	args := []any{t.OrganizationID}

	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if isEmployee(t) {
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM leads l WHERE l.id = tasks.lead_id AND l.organization_id = $1 AND l.assigned_to_membership_id = %s)",
			arg(membershipIDOrNil(t))))
	}
	if filter.AssignedTo != nil {
		conditions = append(conditions, "assigned_to_membership_id = "+arg(*filter.AssignedTo))
	}
	if len(filter.Status) > 0 {
		conditions = append(conditions, "status = ANY("+arg(filter.Status)+")")
	}
	if filter.DueBefore != nil {
		conditions = append(conditions, "due_at <= "+arg(*filter.DueBefore))
	}

	return strings.Join(conditions, " AND "), args
}

// Update applies optimistic locking (TD §4), same shape as
// lead.Update — zero rows disambiguated via a re-run scoped FindByID.
func (r *postgresRepository) Update(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in UpdateInput) (*Task, error) {
	const q = `
		UPDATE tasks
		SET version = version + 1,
		    title       = COALESCE($5, title),
		    description = COALESCE($6, description),
		    due_at      = COALESCE($7, due_at),
		    assigned_to_membership_id = COALESCE($8, assigned_to_membership_id)
		WHERE id = $1 AND organization_id = $2 AND version = $3 AND deleted_at IS NULL
		  AND (NOT $4 OR EXISTS (
		      SELECT 1 FROM leads l WHERE l.id = tasks.lead_id AND l.organization_id = $2
		        AND l.assigned_to_membership_id = $9
		  ))
		RETURNING ` + taskColumns

	row := r.q.QueryRow(ctx, q,
		id, t.OrganizationID, expectedVersion, isEmployee(t),
		in.Title, in.Description, in.DueAt, in.AssignedToMembershipID,
		membershipIDOrNil(t),
	)
	updated, err := scanTask(row)
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
		return nil, fmt.Errorf("task: update: %w", err)
	}
	return updated, nil
}

// Complete sets status/completed_at/completed_by unconditionally on a
// version+scope match — no extra "status must be open" guard.
// Re-completing an already-done task at its current version just
// re-stamps completion; nothing in TD's acceptance criteria exercises
// this edge case either way, so no separate error is invented for it.
func (r *postgresRepository) Complete(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, completedByMembershipID *uuid.UUID) (*Task, error) {
	const q = `
		UPDATE tasks
		SET version = version + 1, status = 'done', completed_at = now(), completed_by_membership_id = $5
		WHERE id = $1 AND organization_id = $2 AND version = $3 AND deleted_at IS NULL
		  AND (NOT $4 OR EXISTS (
		      SELECT 1 FROM leads l WHERE l.id = tasks.lead_id AND l.organization_id = $2
		        AND l.assigned_to_membership_id = $6
		  ))
		RETURNING ` + taskColumns

	row := r.q.QueryRow(ctx, q,
		id, t.OrganizationID, expectedVersion, isEmployee(t),
		completedByMembershipID, membershipIDOrNil(t),
	)
	updated, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		current, findErr := r.FindByID(ctx, t, id)
		if findErr != nil {
			return nil, findErr
		}
		return current, ErrVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("task: complete: %w", err)
	}
	return updated, nil
}

// Delete soft-deletes id within t's scope. No version check — same
// precedent as lead.Delete.
func (r *postgresRepository) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	const q = `
		UPDATE tasks SET deleted_at = now()
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL
		  AND (NOT $3 OR EXISTS (
		      SELECT 1 FROM leads l WHERE l.id = tasks.lead_id AND l.organization_id = $2
		        AND l.assigned_to_membership_id = $4
		  ))`

	tag, err := r.q.Exec(ctx, q, id, t.OrganizationID, isEmployee(t), membershipIDOrNil(t))
	if err != nil {
		return fmt.Errorf("task: delete: %w", err)
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

const taskColumns = `
	id, organization_id, lead_id, title, description, due_at, status,
	assigned_to_membership_id, completed_at, completed_by_membership_id,
	version, created_by_membership_id, created_at, updated_at, deleted_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (*Task, error) {
	var tsk Task
	err := row.Scan(
		&tsk.ID, &tsk.OrganizationID, &tsk.LeadID, &tsk.Title, &tsk.Description, &tsk.DueAt, &tsk.Status,
		&tsk.AssignedToMembershipID, &tsk.CompletedAt, &tsk.CompletedByMembershipID,
		&tsk.Version, &tsk.CreatedByMembershipID, &tsk.CreatedAt, &tsk.UpdatedAt, &tsk.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &tsk, nil
}

func scanTasks(rows pgx.Rows) ([]*Task, error) {
	var out []*Task
	for rows.Next() {
		tsk, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("task: scan: %w", err)
		}
		out = append(out, tsk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: scan: %w", err)
	}
	return out, nil
}
