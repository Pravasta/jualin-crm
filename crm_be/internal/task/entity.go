// Package task is CRM Core's follow-up entity — always attached to a
// lead (lead_id NOT NULL, decision B8). See
// docs/phases/02-crm-core/td.md §1.4, §8, §10.
package task

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	LeadID         uuid.UUID

	Title       string
	Description *string
	DueAt       *time.Time
	Status      string

	AssignedToMembershipID  *uuid.UUID
	CompletedAt             *time.Time
	CompletedByMembershipID *uuid.UUID

	Version               int
	CreatedByMembershipID *uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

const (
	StatusOpen = "open"
	StatusDone = "done"
)

// CreateInput is Repository.Create's argument — no validation here;
// that's the usecase's job.
type CreateInput struct {
	LeadID                 uuid.UUID
	Title                  string
	Description            *string
	DueAt                  *time.Time
	AssignedToMembershipID *uuid.UUID
	CreatedByMembershipID  *uuid.UUID
}

// UpdateInput is the mutable-field subset PATCH /v1/tasks/{id} exposes —
// nil means "don't touch", same limitation lead.UpdateInput already
// documents: this minimal shape has no way to clear a field to NULL.
type UpdateInput struct {
	Title                  *string
	Description            *string
	DueAt                  *time.Time
	AssignedToMembershipID *uuid.UUID
}

// ListFilter is FindAllByOrg's argument — org-wide, paginated (GET
// /v1/tasks). GET /v1/leads/{id}/tasks is a separate repository method,
// FindAllByLead, not a LeadID field here — it needs an explicit
// lead-visibility check (404 when the lead isn't visible), not a filter
// that would just quietly narrow the result set to empty (TD §8's
// acceptance criteria: reading a sub-resource of a lead you can't see
// is a 404).
type ListFilter struct {
	AssignedTo *uuid.UUID
	Status     []string
	DueBefore  *time.Time
	Page       int
	PerPage    int
}

// ErrVersionConflict is what Update/Complete return — alongside the
// row's CURRENT state — when expectedVersion no longer matches. Same
// shape as lead.ErrVersionConflict.
var ErrVersionConflict = errors.New("task: version conflict")

// ErrAssigneeNotFound mirrors lead.ErrAssigneeNotFound — detected via
// the fk_tasks_assignee foreign-key violation, same
// database-constraint-as-source-of-truth pattern. Included proactively
// since it's the identical shape of bug #20 already found once for
// leads, not a speculative addition.
var ErrAssigneeNotFound = errors.New("task: assignee not found")

// VersionConflictError carries the row's current state — same pattern
// as lead.VersionConflictError.
type VersionConflictError struct {
	Current *Task
}

func (e *VersionConflictError) Error() string { return "version conflict" }
