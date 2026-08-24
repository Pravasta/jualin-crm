package activity

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is activity's own table interface — postgresRepository
// (repository_postgres.go) satisfies it, but Usecase depends only on
// this interface (ADR-011).
type Repository interface {
	Create(ctx context.Context, t tenant.Context, in CreateInput) (*Activity, error)
	FindAllByLead(ctx context.Context, t tenant.Context, leadID uuid.UUID) ([]*Activity, error)
}

// Recorder is the narrow surface lead and task need to append a
// system-generated activity from inside their own Store.InTx (TD §10) —
// expressed with only primitive/shared types (no *Activity, no
// CreateInput) so that lead.ActivityRecorder and task.ActivityRecorder,
// each declared locally in their own package per ADR-011, are satisfied
// structurally by whatever NewRecorder returns without either package
// importing this one. Same bridging role as auth.RefreshTokenRevoker
// plays for membership (#11).
type Recorder interface {
	Record(ctx context.Context, t tenant.Context, leadID uuid.UUID, activityType string, actorMembershipID *uuid.UUID, metadata map[string]any) error
}

// Repos bundles what a single Usecase call needs — one field today,
// same shape as every other domain's Unit of Work even though
// activity's own usecase never itself needs a multi-repository
// transaction (kept for composition-root wiring consistency, not extra
// machinery beyond that).
type Repos struct {
	Activity Repository
}

// Store is the Unit of Work Usecase depends on.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
