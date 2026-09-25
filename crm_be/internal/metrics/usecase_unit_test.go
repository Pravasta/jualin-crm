package metrics_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Repository, no Docker. Run in isolation with:
//
//	go test ./internal/metrics/... -run TestUnit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/metrics"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type fakeMetricsRepo struct {
	summary       *metrics.Summary
	employees     []*metrics.EmployeeMetric
	lastFilter    metrics.Filter
	summaryCalled bool
	trendCalled   bool
	lastBucket    metrics.TrendBucket
}

func (f *fakeMetricsRepo) Trend(_ context.Context, _ tenant.Context, filter metrics.Filter, bucket metrics.TrendBucket) (*metrics.Trend, error) {
	f.trendCalled = true
	f.lastFilter = filter
	f.lastBucket = bucket
	return &metrics.Trend{Bucket: bucket}, nil
}

func (f *fakeMetricsRepo) Sources(_ context.Context, _ tenant.Context, filter metrics.Filter) ([]*metrics.SourceMetric, error) {
	f.lastFilter = filter
	return []*metrics.SourceMetric{}, nil
}

func (f *fakeMetricsRepo) LostReasons(_ context.Context, _ tenant.Context, filter metrics.Filter) ([]*metrics.LostReasonMetric, error) {
	f.lastFilter = filter
	return []*metrics.LostReasonMetric{}, nil
}

func (f *fakeMetricsRepo) Summary(_ context.Context, _ tenant.Context, filter metrics.Filter) (*metrics.Summary, error) {
	f.summaryCalled = true
	f.lastFilter = filter
	return f.summary, nil
}

func (f *fakeMetricsRepo) Employees(_ context.Context, _ tenant.Context, filter metrics.Filter) ([]*metrics.EmployeeMetric, error) {
	f.lastFilter = filter
	return f.employees, nil
}

func actorContext(role tenant.Role) tenant.Context {
	membershipID := uuid.Must(uuid.NewV7())
	return tenant.Context{
		OrganizationID: uuid.Must(uuid.NewV7()),
		PrincipalType:  tenant.PrincipalUser,
		MembershipID:   &membershipID,
		Role:           role,
	}
}

func assertForbidden(t *testing.T, err error) {
	t.Helper()
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden, got: %v", err)
	}
}

func TestUnit_Summary_EmployeeForbidden(t *testing.T) {
	repo := &fakeMetricsRepo{summary: &metrics.Summary{}}
	u := metrics.NewUsecase(repo)

	_, err := u.Summary(context.Background(), actorContext(tenant.RoleEmployee), metrics.Filter{})
	assertForbidden(t, err)
	if repo.summaryCalled {
		t.Error("expected repository not to be called for a forbidden actor")
	}
}

func TestUnit_Employees_EmployeeForbidden(t *testing.T) {
	repo := &fakeMetricsRepo{}
	u := metrics.NewUsecase(repo)

	_, err := u.Employees(context.Background(), actorContext(tenant.RoleEmployee), metrics.Filter{})
	assertForbidden(t, err)
}

func TestUnit_Summary_OwnerAdminManagerAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin, tenant.RoleManager} {
		t.Run(string(role), func(t *testing.T) {
			repo := &fakeMetricsRepo{summary: &metrics.Summary{TotalNew: 5}}
			u := metrics.NewUsecase(repo)

			got, err := u.Summary(context.Background(), actorContext(role), metrics.Filter{})
			if err != nil {
				t.Fatalf("expected %s to be allowed, got: %v", role, err)
			}
			if got.TotalNew != 5 {
				t.Errorf("expected summary passed through from repository, got %+v", got)
			}
		})
	}
}

func TestUnit_Summary_PassesFilterThrough(t *testing.T) {
	repo := &fakeMetricsRepo{summary: &metrics.Summary{}}
	u := metrics.NewUsecase(repo)

	from := mustParseTime(t, "2026-01-01T00:00:00Z")
	to := mustParseTime(t, "2026-01-31T23:59:59Z")
	filter := metrics.Filter{From: &from, To: &to}

	if _, err := u.Summary(context.Background(), actorContext(tenant.RoleOwner), filter); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastFilter.From == nil || !repo.lastFilter.From.Equal(from) {
		t.Errorf("expected From to reach the repository unchanged, got %v", repo.lastFilter.From)
	}
	if repo.lastFilter.To == nil || !repo.lastFilter.To.Equal(to) {
		t.Errorf("expected To to reach the repository unchanged, got %v", repo.lastFilter.To)
	}
}

func TestUnit_Summary_ConversionRateNilWhenDenominatorZero(t *testing.T) {
	// This test proves the USECASE doesn't invent a value — the nil-vs-0
	// distinction itself is the repository's job (TD §2.2), covered
	// against real Postgres in repository_test.go. Here we only prove
	// the usecase passes whatever the repository computed straight
	// through, without collapsing nil to 0 anywhere in between.
	repo := &fakeMetricsRepo{summary: &metrics.Summary{ConversionRate: nil}}
	u := metrics.NewUsecase(repo)

	got, err := u.Summary(context.Background(), actorContext(tenant.RoleOwner), metrics.Filter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ConversionRate != nil {
		t.Errorf("expected nil conversion_rate to remain nil, got %v", *got.ConversionRate)
	}
}

func TestUnit_Employees_OwnerAllowed(t *testing.T) {
	repo := &fakeMetricsRepo{employees: []*metrics.EmployeeMetric{{FullName: "Budi"}}}
	u := metrics.NewUsecase(repo)

	got, err := u.Employees(context.Background(), actorContext(tenant.RoleOwner), metrics.Filter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].FullName != "Budi" {
		t.Errorf("expected employees passed through from repository, got %+v", got)
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}

// --- Phase 8.6 (#169): trend, sources, lost reasons ---------------------

func rangeOf(days int) (*time.Time, *time.Time) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Duration(days) * 24 * time.Hour)
	return &from, &to
}

func validationFields(t *testing.T, err error) map[string]string {
	t.Helper()
	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected a validation error, got: %v", err)
	}
	out := map[string]string{}
	for _, d := range verr.Details {
		out[d.Field] = d.Code
	}
	return out
}

func TestUnit_Trend_RequiresBothBounds(t *testing.T) {
	repo := &fakeMetricsRepo{}
	u := metrics.NewUsecase(repo)

	_, err := u.Trend(context.Background(), actorContext(tenant.RoleOwner), metrics.Filter{})
	got := validationFields(t, err)
	if got["from"] != "required" || got["to"] != "required" {
		t.Errorf("expected from/to required, got %v", got)
	}
	if repo.trendCalled {
		t.Error("an unbounded range must never reach generate_series")
	}
}

func TestUnit_Trend_RejectsBackwardsAndOverlongRanges(t *testing.T) {
	for name, days := range map[string]int{"backwards": -1, "over 366 days": 367} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeMetricsRepo{}
			from, to := rangeOf(days)
			_, err := metrics.NewUsecase(repo).Trend(context.Background(), actorContext(tenant.RoleOwner), metrics.Filter{From: from, To: to})
			if got := validationFields(t, err); got["to"] != "invalid_value" {
				t.Errorf("expected to=invalid_value, got %v", got)
			}
			if repo.trendCalled {
				t.Error("repository must not be called for an invalid range")
			}
		})
	}
}

// The server, not the client, picks the bucket: days up to 45, then weeks.
func TestUnit_Trend_PicksBucketFromRange(t *testing.T) {
	cases := []struct {
		days int
		want metrics.TrendBucket
	}{{0, metrics.TrendBucketDay}, {45, metrics.TrendBucketDay}, {46, metrics.TrendBucketWeek}, {366, metrics.TrendBucketWeek}}
	for _, tc := range cases {
		repo := &fakeMetricsRepo{}
		from, to := rangeOf(tc.days)
		if _, err := metrics.NewUsecase(repo).Trend(context.Background(), actorContext(tenant.RoleManager), metrics.Filter{From: from, To: to}); err != nil {
			t.Fatalf("%d days: %v", tc.days, err)
		}
		if repo.lastBucket != tc.want {
			t.Errorf("%d days: expected bucket %q, got %q", tc.days, tc.want, repo.lastBucket)
		}
	}
}

func TestUnit_NewReports_EmployeeForbidden(t *testing.T) {
	from, to := rangeOf(30)
	f := metrics.Filter{From: from, To: to}
	actor := actorContext(tenant.RoleEmployee)
	u := metrics.NewUsecase(&fakeMetricsRepo{})

	_, err := u.Trend(context.Background(), actor, f)
	assertForbidden(t, err)
	_, err = u.Sources(context.Background(), actor, f)
	assertForbidden(t, err)
	_, err = u.LostReasons(context.Background(), actor, f)
	assertForbidden(t, err)
}

func TestUnit_NewReports_OwnerAdminManagerAllowed(t *testing.T) {
	from, to := rangeOf(30)
	f := metrics.Filter{From: from, To: to}
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin, tenant.RoleManager} {
		t.Run(string(role), func(t *testing.T) {
			u := metrics.NewUsecase(&fakeMetricsRepo{})
			actor := actorContext(role)
			if _, err := u.Trend(context.Background(), actor, f); err != nil {
				t.Errorf("trend: %v", err)
			}
			if _, err := u.Sources(context.Background(), actor, f); err != nil {
				t.Errorf("sources: %v", err)
			}
			if _, err := u.LostReasons(context.Background(), actor, f); err != nil {
				t.Errorf("lost reasons: %v", err)
			}
		})
	}
}
