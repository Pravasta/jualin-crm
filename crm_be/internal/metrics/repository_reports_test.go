package metrics_test

// Phase 8.6 (#169) — repository tests for /trend, /sources, /lost-reasons
// and the assignee/source filter, against real PostgreSQL (dbtest).
// Includes the mandatory tenant-isolation case for every new query (Aturan #7).

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/metrics"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func seedLeadFull(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, assignedTo *uuid.UUID, status, source string, lostReason *string, createdAt time.Time) {
	t.Helper()
	const q = `INSERT INTO leads (id, organization_id, lead_number, name, source, assigned_to_membership_id, status, lost_reason, created_at)
		VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), 'Test Lead', $3, $4, $5, $6, $7)`
	if _, err := pool.Exec(ctx, q, uuid.Must(uuid.NewV7()), org, source, assignedTo, status, lostReason, createdAt); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE organizations SET next_lead_number = next_lead_number + 1 WHERE id = $1`, org); err != nil {
		t.Fatalf("bump next_lead_number: %v", err)
	}
}

func setTimezone(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, tz string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE organizations SET timezone = $2 WHERE id = $1`, org, tz); err != nil {
		t.Fatalf("set timezone: %v", err)
	}
}

func day(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }

func ptr[T any](v T) *T { return &v }

// Every day in the range comes back — including the ones with no leads —
// and each lead lands on its own day.
func TestRepository_Trend_DailyIncludesEmptyBuckets(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	setTimezone(t, ctx, pool, org, "UTC")

	seedLeadAt(t, ctx, pool, org, nil, "new", day(2026, 1, 1, 10))
	seedLeadAt(t, ctx, pool, org, nil, "new", day(2026, 1, 1, 11))
	seedLeadAt(t, ctx, pool, org, nil, "new", day(2026, 1, 3, 9))

	from, to := day(2026, 1, 1, 0), time.Date(2026, 1, 4, 23, 59, 59, 0, time.UTC)
	tr, err := repo.Trend(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{From: &from, To: &to}, metrics.TrendBucketDay)
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	want := []struct {
		date  string
		count int
	}{{"2026-01-01", 2}, {"2026-01-02", 0}, {"2026-01-03", 1}, {"2026-01-04", 0}}
	if len(tr.Points) != len(want) {
		t.Fatalf("expected %d points (empty days included), got %d: %+v", len(want), len(tr.Points), tr.Points)
	}
	for i, w := range want {
		if got := tr.Points[i].Date.Format("2006-01-02"); got != w.date || tr.Points[i].Count != w.count {
			t.Errorf("point %d: expected %s=%d, got %s=%d", i, w.date, w.count, got, tr.Points[i].Count)
		}
	}
}

// Aturan #13: days are the ORGANIZATION's days. 16:00 UTC on 15 Jan is
// 01:00 on 16 Jan in Jayapura (UTC+9) — it must count on the 16th there.
func TestRepository_Trend_UsesOrganizationTimezone(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	setTimezone(t, ctx, pool, org, "Asia/Jayapura")

	seedLeadAt(t, ctx, pool, org, nil, "new", day(2026, 1, 15, 16))

	from, to := day(2026, 1, 14, 15), day(2026, 1, 16, 14) // 15–16 Jan, Jayapura calendar
	tr, err := repo.Trend(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{From: &from, To: &to}, metrics.TrendBucketDay)
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	counts := map[string]int{}
	for _, p := range tr.Points {
		counts[p.Date.Format("2006-01-02")] = p.Count
	}
	if counts["2026-01-16"] != 1 || counts["2026-01-15"] != 0 {
		t.Fatalf("expected the lead on 16 Jan (Jayapura), got %v — bucketed in UTC", counts)
	}
}

// Week buckets start on Monday. 5 Jan 2026 is a Monday.
func TestRepository_Trend_WeeklyStartsMonday(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	setTimezone(t, ctx, pool, org, "UTC")

	seedLeadAt(t, ctx, pool, org, nil, "new", day(2026, 1, 11, 12)) // Sunday → week of 5 Jan
	seedLeadAt(t, ctx, pool, org, nil, "new", day(2026, 1, 12, 12)) // Monday → week of 12 Jan

	from, to := day(2026, 1, 5, 0), day(2026, 1, 18, 23)
	tr, err := repo.Trend(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{From: &from, To: &to}, metrics.TrendBucketWeek)
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	if len(tr.Points) != 2 || tr.Points[0].Date.Format("2006-01-02") != "2026-01-05" || tr.Points[0].Count != 1 ||
		tr.Points[1].Date.Format("2006-01-02") != "2026-01-12" || tr.Points[1].Count != 1 {
		t.Fatalf("expected weeks of 5 Jan (1) and 12 Jan (1), got %+v", tr.Points)
	}
}

// Four rows always, and the same conversion-rate definition as /summary.
func TestRepository_Sources_AllFourWithSummaryDefinition(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	now := time.Now().UTC()

	seedLeadFull(t, ctx, pool, org, nil, "won", "form", nil, now)
	seedLeadFull(t, ctx, pool, org, nil, "new", "form", nil, now)
	seedLeadFull(t, ctx, pool, org, nil, "spam", "form", nil, now)
	seedLeadFull(t, ctx, pool, org, nil, "spam", "api", nil, now)

	out, err := repo.Sources(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{})
	if err != nil {
		t.Fatalf("sources: %v", err)
	}
	if len(out) != 4 || out[0].Source != "manual" || out[1].Source != "api" || out[2].Source != "form" || out[3].Source != "webhook" {
		t.Fatalf("expected manual/api/form/webhook in that order, got %+v", out)
	}
	form := out[2]
	// 3 form leads, 1 spam → denominator 2, 1 won → 0.5 — spam leaves the denominator.
	if form.Count != 3 || form.WonCount != 1 || form.ConversionRate == nil || *form.ConversionRate != 0.5 {
		t.Errorf("form: expected count=3 won=1 rate=0.5, got count=%d won=%d rate=%v", form.Count, form.WonCount, form.ConversionRate)
	}
	// api: only a spam lead → denominator 0 → nil, never a bare 0.
	if out[1].Count != 1 || out[1].ConversionRate != nil {
		t.Errorf("api: expected count=1 and nil rate, got count=%d rate=%v", out[1].Count, out[1].ConversionRate)
	}
	if out[0].Count != 0 || out[3].Count != 0 {
		t.Errorf("unused sources must come back as zero, got manual=%d webhook=%d", out[0].Count, out[3].Count)
	}
}

func TestRepository_LostReasons_AllSixOnlyLostLeads(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	now := time.Now().UTC()

	seedLeadFull(t, ctx, pool, org, nil, "lost", "manual", ptr("price"), now)
	seedLeadFull(t, ctx, pool, org, nil, "lost", "manual", ptr("price"), now)
	seedLeadFull(t, ctx, pool, org, nil, "lost", "manual", ptr("timing"), now)
	seedLeadFull(t, ctx, pool, org, nil, "new", "manual", nil, now)

	out, err := repo.LostReasons(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{})
	if err != nil {
		t.Fatalf("lost reasons: %v", err)
	}
	got := map[string]int{}
	for _, m := range out {
		got[m.Reason] = m.Count
	}
	if len(out) != 6 || got["price"] != 2 || got["timing"] != 1 || got["other"] != 0 {
		t.Fatalf("expected six reasons with price=2 timing=1, got %v", got)
	}
}

// The new filter narrows every endpoint, including /summary.
func TestRepository_AssigneeAndSourceFilter(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	andi := seedMembership(t, ctx, pool, org, "andi@example.com", "Andi")
	sari := seedMembership(t, ctx, pool, org, "sari@example.com", "Sari")
	now := time.Now().UTC()

	seedLeadFull(t, ctx, pool, org, &andi, "new", "form", nil, now)
	seedLeadFull(t, ctx, pool, org, &andi, "new", "api", nil, now)
	seedLeadFull(t, ctx, pool, org, &sari, "new", "form", nil, now)

	tc := tenant.Context{OrganizationID: org}
	s, err := repo.Summary(ctx, tc, metrics.Filter{Assignee: &andi})
	if err != nil || s.TotalNew != 2 {
		t.Fatalf("assignee filter on summary: expected 2, got %v (err %v)", s, err)
	}
	s, err = repo.Summary(ctx, tc, metrics.Filter{Source: ptr("form")})
	if err != nil || s.TotalNew != 2 {
		t.Fatalf("source filter on summary: expected 2, got %v (err %v)", s, err)
	}
	s, err = repo.Summary(ctx, tc, metrics.Filter{Assignee: &andi, Source: ptr("form")})
	if err != nil || s.TotalNew != 1 {
		t.Fatalf("combined filter: expected 1, got %v (err %v)", s, err)
	}

	em, err := repo.Employees(ctx, tc, metrics.Filter{Assignee: &andi})
	if err != nil {
		t.Fatalf("employees: %v", err)
	}
	if len(em) != 1 || em[0].MembershipID != andi || em[0].LeadCount != 2 {
		t.Fatalf("assignee filter on employees: expected only Andi with 2 leads, got %+v", em)
	}
}

// Aturan #7 for every new query, and the filter can't be used to peek:
// a membership id from another tenant narrows to nothing, it doesn't error.
func TestRepository_Reports_ScopedToOrganization(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	setTimezone(t, ctx, pool, orgA, "UTC")
	memberB := seedMembership(t, ctx, pool, orgB, "b@example.com", "B")
	now := time.Now().UTC()

	seedLeadFull(t, ctx, pool, orgA, nil, "lost", "form", ptr("price"), now)
	seedLeadFull(t, ctx, pool, orgB, &memberB, "lost", "form", ptr("price"), now)
	seedLeadFull(t, ctx, pool, orgB, &memberB, "lost", "form", ptr("price"), now)

	a := tenant.Context{OrganizationID: orgA}
	from, to := now.Add(-time.Hour), now.Add(time.Hour)

	tr, err := repo.Trend(ctx, a, metrics.Filter{From: &from, To: &to}, metrics.TrendBucketDay)
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	total := 0
	for _, p := range tr.Points {
		total += p.Count
	}
	if total != 1 {
		t.Errorf("trend leaked another org's leads: total=%d", total)
	}

	src, err := repo.Sources(ctx, a, metrics.Filter{})
	if err != nil || src[2].Count != 1 {
		t.Errorf("sources leaked another org's leads: %+v (err %v)", src, err)
	}
	lr, err := repo.LostReasons(ctx, a, metrics.Filter{})
	if err != nil || lr[0].Count != 1 {
		t.Errorf("lost reasons leaked another org's leads: %+v (err %v)", lr, err)
	}

	s, err := repo.Summary(ctx, a, metrics.Filter{Assignee: &memberB})
	if err != nil {
		t.Fatalf("summary with foreign assignee: %v", err)
	}
	if s.TotalNew != 0 {
		t.Errorf("a foreign membership id must narrow to nothing, got total_new=%d", s.TotalNew)
	}
}
