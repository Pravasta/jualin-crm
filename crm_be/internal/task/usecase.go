package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const (
	defaultPerPage = 25
	maxPerPage     = 100
)

// Usecase depends only on Store (port.go), never on *pgxpool.Pool or
// pgx.Tx directly (ADR-011).
type Usecase struct {
	store Store
}

func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store}
}

// CreateTaskInput is Usecase.Create's argument.
type CreateTaskInput struct {
	Title                  string
	Description            *string
	DueAt                  *time.Time
	AssignedToMembershipID *uuid.UUID
}

// Create makes a task on leadID and records task_created in the SAME
// transaction (TD §10) — atomicity is the point: a task that exists
// without a timeline entry, or an activity entry for a task that was
// rolled back, are both worse than neither existing.
func (u *Usecase) Create(ctx context.Context, t tenant.Context, leadID uuid.UUID, in CreateTaskInput) (*Task, error) {
	if err := authz.Require(t, authz.ActionTaskCreate); err != nil {
		return nil, err
	}
	if in.Title == "" {
		return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "title", Code: "required"})
	}

	repoIn := CreateInput{
		LeadID: leadID, Title: in.Title, Description: in.Description, DueAt: in.DueAt,
		AssignedToMembershipID: in.AssignedToMembershipID, CreatedByMembershipID: t.MembershipID,
	}

	var created *Task
	txErr := u.store.InTx(ctx, func(r Repos) error {
		c, err := r.Task.Create(ctx, t, repoIn)
		if err != nil {
			return err
		}
		created = c
		return r.Activity.Record(ctx, t, leadID, "task_created", t.MembershipID, map[string]any{
			"task_id": c.ID, "title": c.Title,
		})
	})
	if txErr != nil {
		if errors.Is(txErr, ErrAssigneeNotFound) {
			return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "assigned_to_membership_id", Code: "not_found"})
		}
		return nil, fmt.Errorf("task: create: %w", txErr)
	}
	return created, nil
}

// ListTasksInput is ListByOrg's argument — raw query-param values;
// ListByOrg clamps Page/PerPage before delegating to the repository.
type ListTasksInput struct {
	AssignedTo *uuid.UUID
	Status     []string
	DueBefore  *time.Time
	Page       int
	PerPage    int
}

// ListByOrg backs GET /v1/tasks.
func (u *Usecase) ListByOrg(ctx context.Context, t tenant.Context, in ListTasksInput) ([]*Task, httpx.Meta, error) {
	if err := authz.Require(t, authz.ActionTaskRead); err != nil {
		return nil, httpx.Meta{}, err
	}

	page := in.Page
	if page < 1 {
		page = 1
	}
	perPage := in.PerPage
	if perPage <= 0 {
		perPage = defaultPerPage
	} else if perPage > maxPerPage {
		perPage = maxPerPage
	}

	filter := ListFilter{
		AssignedTo: in.AssignedTo, Status: in.Status, DueBefore: in.DueBefore,
		Page: page, PerPage: perPage,
	}
	tasks, total, err := u.store.Repos().Task.FindAllByOrg(ctx, t, filter)
	if err != nil {
		return nil, httpx.Meta{}, fmt.Errorf("task: list by org: %w", err)
	}
	return tasks, httpx.Meta{Page: page, PerPage: perPage, Total: total}, nil
}

// ListByLead backs GET /v1/leads/{id}/tasks — unpaginated (see
// ListFilter's doc comment).
func (u *Usecase) ListByLead(ctx context.Context, t tenant.Context, leadID uuid.UUID) ([]*Task, error) {
	if err := authz.Require(t, authz.ActionTaskRead); err != nil {
		return nil, err
	}
	return u.store.Repos().Task.FindAllByLead(ctx, t, leadID)
}

// UpdateTaskInput is Usecase.Update's argument.
type UpdateTaskInput struct {
	Version                int
	Title                  *string
	Description            *string
	DueAt                  *time.Time
	AssignedToMembershipID *uuid.UUID
}

// Update applies general field changes — no activity type exists for
// "task updated" in ck_activities_type (TD §10's table), so none is
// written here, unlike Create/Complete.
func (u *Usecase) Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateTaskInput) (*Task, error) {
	if err := authz.Require(t, authz.ActionTaskUpdate); err != nil {
		return nil, err
	}

	repoIn := UpdateInput{
		Title: in.Title, Description: in.Description, DueAt: in.DueAt,
		AssignedToMembershipID: in.AssignedToMembershipID,
	}
	updated, err := u.store.Repos().Task.Update(ctx, t, id, in.Version, repoIn)
	if errors.Is(err, ErrVersionConflict) {
		return nil, &VersionConflictError{Current: updated}
	}
	if errors.Is(err, ErrAssigneeNotFound) {
		return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "assigned_to_membership_id", Code: "not_found"})
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Complete marks id done and records task_completed in the SAME
// transaction (TD §10) — same atomicity reasoning as Create.
func (u *Usecase) Complete(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int) (*Task, error) {
	if err := authz.Require(t, authz.ActionTaskComplete); err != nil {
		return nil, err
	}

	var completed *Task
	var conflict bool
	txErr := u.store.InTx(ctx, func(r Repos) error {
		c, err := r.Task.Complete(ctx, t, id, expectedVersion, t.MembershipID)
		if errors.Is(err, ErrVersionConflict) {
			completed = c
			conflict = true
			return ErrVersionConflict
		}
		if err != nil {
			return err
		}
		completed = c
		return r.Activity.Record(ctx, t, c.LeadID, "task_completed", t.MembershipID, map[string]any{
			"task_id": c.ID,
		})
	})
	if conflict {
		return nil, &VersionConflictError{Current: completed}
	}
	if txErr != nil {
		return nil, fmt.Errorf("task: complete: %w", txErr)
	}
	return completed, nil
}

// Delete soft-deletes id — Employee is deliberately excluded by
// authz.ActionTaskDelete (TD §9's matrix, the one place this issue
// denies Employee something). No activity — no "task deleted" type
// exists.
func (u *Usecase) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	if err := authz.Require(t, authz.ActionTaskDelete); err != nil {
		return err
	}
	return u.store.Repos().Task.Delete(ctx, t, id)
}
