package metrics

import (
	"context"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is metrics's own read-only interface — postgresRepository
// (repository_postgres.go) satisfies it, but Usecase depends only on
// this interface (ADR-011). No Store here (see entity.go doc comment):
// this package never writes.
type Repository interface {
	Summary(ctx context.Context, t tenant.Context, filter Filter) (*Summary, error)
	Employees(ctx context.Context, t tenant.Context, filter Filter) ([]*EmployeeMetric, error)
}
