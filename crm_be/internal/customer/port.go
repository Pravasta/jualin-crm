package customer

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is customer's own table interface — postgresRepository
// (repository_postgres.go) satisfies it, but Usecase depends only on
// this interface (ADR-011).
type Repository interface {
	Convert(ctx context.Context, t tenant.Context, leadID uuid.UUID, convertedByMembershipID *uuid.UUID) (*Customer, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Customer, error)
	FindAllByOrg(ctx context.Context, t tenant.Context, filter ListFilter) ([]*Customer, int, error)
	Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Customer, error)
	Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error
}

// ActivityRecorder is declared locally per ADR-011 — same three-line
// shape lead's, task's, and membership's own local interfaces already
// use, independently declared each time rather than shared (ADR-011:
// the interface belongs to the consumer). Satisfied by
// activity.NewRecorder's return value at the composition root.
type ActivityRecorder interface {
	Record(ctx context.Context, t tenant.Context, leadID uuid.UUID, activityType string, actorMembershipID *uuid.UUID, metadata map[string]any) error
}

// Repos bundles what a single Usecase call needs.
type Repos struct {
	Customer Repository
	Activity ActivityRecorder
}

// Store is the Unit of Work Usecase depends on — required because
// Convert writes a customer row and an activity row atomically (TD
// §12).
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
