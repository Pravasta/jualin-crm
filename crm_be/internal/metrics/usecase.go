package metrics

import (
	"context"
	"fmt"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Usecase depends only on Repository (port.go), never on *pgxpool.Pool
// or pgx directly (ADR-011). It takes Repository directly rather than a
// Store — there is no transaction to open.
type Usecase struct {
	repo Repository
}

func NewUsecase(repo Repository) *Usecase {
	return &Usecase{repo: repo}
}

// Summary gates on ActionMetricsRead — Employee never reaches
// Repository.Summary, so the repository itself has no isEmployee branch
// (TD §2.4), unlike lead/task/customer.
func (u *Usecase) Summary(ctx context.Context, t tenant.Context, filter Filter) (*Summary, error) {
	if err := authz.Require(t, authz.ActionMetricsRead); err != nil {
		return nil, err
	}
	s, err := u.repo.Summary(ctx, t, filter)
	if err != nil {
		return nil, fmt.Errorf("metrics: summary: %w", err)
	}
	return s, nil
}

func (u *Usecase) Employees(ctx context.Context, t tenant.Context, filter Filter) ([]*EmployeeMetric, error) {
	if err := authz.Require(t, authz.ActionMetricsRead); err != nil {
		return nil, err
	}
	out, err := u.repo.Employees(ctx, t, filter)
	if err != nil {
		return nil, fmt.Errorf("metrics: employees: %w", err)
	}
	return out, nil
}
