package task

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is task's own table interface — postgresRepository
// (repository_postgres.go) satisfies it, but Usecase depends only on
// this interface (ADR-011).
type Repository interface {
	Create(ctx context.Context, t tenant.Context, in CreateInput) (*Task, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Task, error)
	FindAllByOrg(ctx context.Context, t tenant.Context, filter ListFilter) ([]*Task, int, error)
	FindAllByLead(ctx context.Context, t tenant.Context, leadID uuid.UUID) ([]*Task, error)
	Update(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in UpdateInput) (*Task, error)
	Complete(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, completedByMembershipID *uuid.UUID) (*Task, error)
	Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error
}

// ActivityRecorder is declared locally per ADR-011 — task needs only to
// append a system-generated activity, not activity's full domain type.
// Structurally identical to lead.ActivityRecorder (both declared
// independently, not shared — three lines, not worth a common package
// for two call sites) and satisfied by activity.NewRecorder's return
// value at the composition root.
type ActivityRecorder interface {
	Record(ctx context.Context, t tenant.Context, leadID uuid.UUID, activityType string, actorMembershipID *uuid.UUID, metadata map[string]any) error
}

// Repos bundles what a single Usecase call needs.
type Repos struct {
	Task     Repository
	Activity ActivityRecorder
}

// Store is the Unit of Work Usecase depends on — required because
// Create and Complete each write a task row and an activity row
// atomically (TD §10).
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
