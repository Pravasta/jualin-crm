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
