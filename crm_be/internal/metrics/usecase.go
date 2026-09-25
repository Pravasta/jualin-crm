package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
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

// Trend range rules (Phase 8.6 TD §2.2). Unlike every other endpoint here,
// both bounds are REQUIRED: an unbounded range is an unbounded
// generate_series. Over 45 days the buckets become weeks, so a year is 53
// points, not 366.
const (
	trendMaxRange     = 366 * 24 * time.Hour
	trendDailyMaxSpan = 45 * 24 * time.Hour
)

// Trend validates the range before touching the repository, and picks the
// bucket width itself — the client never chooses it.
func (u *Usecase) Trend(ctx context.Context, t tenant.Context, filter Filter) (*Trend, error) {
	if err := authz.Require(t, authz.ActionMetricsRead); err != nil {
		return nil, err
	}

	var details []httpx.ErrorDetail
	if filter.From == nil {
		details = append(details, httpx.ErrorDetail{Field: "from", Code: "required"})
	}
	if filter.To == nil {
		details = append(details, httpx.ErrorDetail{Field: "to", Code: "required"})
	}
	if len(details) == 0 {
		span := filter.To.Sub(*filter.From)
		if span < 0 || span > trendMaxRange {
			details = append(details, httpx.ErrorDetail{Field: "to", Code: "invalid_value"})
		}
	}
	if len(details) > 0 {
		return nil, httpx.NewValidationError(details...)
	}

	bucket := TrendBucketDay
	if filter.To.Sub(*filter.From) > trendDailyMaxSpan {
		bucket = TrendBucketWeek
	}

	out, err := u.repo.Trend(ctx, t, filter, bucket)
	if err != nil {
		return nil, fmt.Errorf("metrics: trend: %w", err)
	}
	return out, nil
}

func (u *Usecase) Sources(ctx context.Context, t tenant.Context, filter Filter) ([]*SourceMetric, error) {
	if err := authz.Require(t, authz.ActionMetricsRead); err != nil {
		return nil, err
	}
	out, err := u.repo.Sources(ctx, t, filter)
	if err != nil {
		return nil, fmt.Errorf("metrics: sources: %w", err)
	}
	return out, nil
}

func (u *Usecase) LostReasons(ctx context.Context, t tenant.Context, filter Filter) ([]*LostReasonMetric, error) {
	if err := authz.Require(t, authz.ActionMetricsRead); err != nil {
		return nil, err
	}
	out, err := u.repo.LostReasons(ctx, t, filter)
	if err != nil {
		return nil, fmt.Errorf("metrics: lost reasons: %w", err)
	}
	return out, nil
}

func (u *Usecase) ResponseTimes(ctx context.Context, t tenant.Context, filter Filter) (*ResponseTimes, error) {
	if err := authz.Require(t, authz.ActionMetricsRead); err != nil {
		return nil, err
	}
	out, err := u.repo.ResponseTimes(ctx, t, filter)
	if err != nil {
		return nil, fmt.Errorf("metrics: response times: %w", err)
	}
	return out, nil
}

func (u *Usecase) Tasks(ctx context.Context, t tenant.Context, filter Filter) ([]*TaskMetric, error) {
	if err := authz.Require(t, authz.ActionMetricsRead); err != nil {
		return nil, err
	}
	out, err := u.repo.Tasks(ctx, t, filter)
	if err != nil {
		return nil, fmt.Errorf("metrics: tasks: %w", err)
	}
	return out, nil
}
