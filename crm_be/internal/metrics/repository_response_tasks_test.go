package metrics_test

// Phase 8.6 (#170) — /response-times and /tasks against real PostgreSQL.

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

type taskSeed struct {
	assignee    uuid.UUID
	status      string
	dueAt       *time.Time
	completedAt *time.Time
	deleted     bool
}

func seedTask(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, leadID uuid.UUID, s taskSeed) {
	t.Helper()
	var deletedAt *time.Time
	if s.deleted {
		now := time.Now().UTC()
		deletedAt = &now
	}
	const q = `INSERT INTO tasks (id, organization_id, lead_id, title, status, due_at, completed_at, assigned_to_membership_id, deleted_at)
		VALUES ($1, $2, $3, 'Test Task', $4, $5, $6, $7, $8)`
	if _, err := pool.Exec(ctx, q, uuid.Must(uuid.NewV7()), org, leadID, s.status, s.dueAt, s.completedAt, s.assignee, deletedAt); err != nil {
		t.Fatalf("seed task: %v", err)
	}
}

func bucketCounts(rt *metrics.ResponseTimes) map[string]int {
	out := map[string]int{}
	for _, b := range rt.Buckets {
		out[b.Bucket] = b.Count
	}
	return out
}

// Boundaries are lower-bound inclusive: exactly 1h → 1h_4h, exactly 4h →
// 4h_24h, exactly 24h → gt_24h. Only lead_created doesn't count as a touch,
// and an untouched lead is never_touched — never gt_24h.
func TestRepository_ResponseTimes_BucketBoundariesAndNeverTouched(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	base := time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC)

	for _, after := range []time.Duration{59 * time.Minute, time.Hour, 4 * time.Hour, 24 * time.Hour} {
		lead := seedLeadAt(t, ctx, pool, org, nil, "contacted", base)
		seedActivity(t, ctx, pool, org, lead, "lead_created", base)
		seedActivity(t, ctx, pool, org, lead, "note_added", base.Add(after))
		seedActivity(t, ctx, pool, org, lead, "call_logged", base.Add(after+48*time.Hour)) // later touches don't matter
	}
	untouched := seedLeadAt(t, ctx, pool, org, nil, "new", base)
	seedActivity(t, ctx, pool, org, untouched, "lead_created", base) // lead_created is not a touch
	seedLeadAt(t, ctx, pool, org, nil, "new", base)                  // no activity at all

	rt, err := repo.ResponseTimes(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{})
	if err != nil {
		t.Fatalf("response times: %v", err)
	}
	got := bucketCounts(rt)
	want := map[string]int{"lt_1h": 1, "1h_4h": 1, "4h_24h": 1, "gt_24h": 1, "never_touched": 2}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("bucket %s: expected %d, got %d (all: %v)", k, v, got[k], got)
		}
	}
	if len(rt.Buckets) != 5 || rt.Buckets[0].Bucket != "lt_1h" || rt.Buckets[4].Bucket != "never_touched" {
		t.Errorf("expected the five buckets in fixed order, got %+v", rt.Buckets)
	}
	// Median over TOUCHED leads only: 3540, 3600, 14400, 86400 → (3600+14400)/2.
	if rt.MedianSeconds == nil || *rt.MedianSeconds != 9000 {
		t.Errorf("expected median 9000s over touched leads, got %v", rt.MedianSeconds)
	}
}

func TestRepository_ResponseTimes_MedianNilWhenNothingTouched(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	seedLeadAt(t, ctx, pool, org, nil, "new", time.Now().UTC())

	rt, err := repo.ResponseTimes(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{})
	if err != nil {
		t.Fatalf("response times: %v", err)
	}
	if rt.MedianSeconds != nil {
		t.Errorf("expected nil median when no lead was ever touched, got %v", *rt.MedianSeconds)
	}
	if bucketCounts(rt)["never_touched"] != 1 {
		t.Errorf("expected the lead in never_touched, got %+v", rt.Buckets)
	}
}

// completed_count follows the range; overdue_count is a snapshot of now and
// ignores it (TD §2.6). Deleted tasks and tasks on deleted leads don't count.
func TestRepository_Tasks_CompletedInRangeOverdueNow(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	andi := seedMembership(t, ctx, pool, org, "andi@example.com", "Andi")
	seedMembership(t, ctx, pool, org, "sari@example.com", "Sari") // no tasks — still listed, zeros

	lead := seedLeadAt(t, ctx, pool, org, nil, "contacted", time.Now().UTC())
	deletedLead := seedLeadAt(t, ctx, pool, org, nil, "contacted", time.Now().UTC())
	if _, err := pool.Exec(ctx, `UPDATE leads SET deleted_at = now() WHERE id = $1`, deletedLead); err != nil {
		t.Fatalf("delete lead: %v", err)
	}

	inRange := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	outOfRange := time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)
	past := time.Now().UTC().Add(-48 * time.Hour)
	future := time.Now().UTC().Add(48 * time.Hour)

	seedTask(t, ctx, pool, org, lead, taskSeed{assignee: andi, status: "done", completedAt: &inRange})
	seedTask(t, ctx, pool, org, lead, taskSeed{assignee: andi, status: "done", completedAt: &outOfRange})
	seedTask(t, ctx, pool, org, lead, taskSeed{assignee: andi, status: "open", dueAt: &past})
	seedTask(t, ctx, pool, org, lead, taskSeed{assignee: andi, status: "open", dueAt: &future})
	seedTask(t, ctx, pool, org, lead, taskSeed{assignee: andi, status: "open"}) // no due date — never overdue
	seedTask(t, ctx, pool, org, lead, taskSeed{assignee: andi, status: "open", dueAt: &past, deleted: true})
	seedTask(t, ctx, pool, org, deletedLead, taskSeed{assignee: andi, status: "open", dueAt: &past})

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)
	out, err := repo.Tasks(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{From: &from, To: &to})
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected both active members listed, got %d", len(out))
	}
	byName := map[string]*metrics.TaskMetric{}
	for _, m := range out {
		byName[m.FullName] = m
	}
	if a := byName["Andi"]; a.CompletedCount != 1 || a.OverdueCount != 1 {
		t.Errorf("Andi: expected completed=1 (in range only), overdue=1 (not deleted, lead not deleted), got completed=%d overdue=%d",
			a.CompletedCount, a.OverdueCount)
	}
	if s := byName["Sari"]; s.CompletedCount != 0 || s.OverdueCount != 0 {
		t.Errorf("Sari: expected zeros, got %+v", s)
	}
}

func TestRepository_Tasks_FiltersAndIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	andi := seedMembership(t, ctx, pool, orgA, "andi@example.com", "Andi")
	seedMembership(t, ctx, pool, orgA, "sari@example.com", "Sari")
	memberB := seedMembership(t, ctx, pool, orgB, "b@example.com", "B")
	past := time.Now().UTC().Add(-time.Hour)
	now := time.Now().UTC()

	seedLeadFull(t, ctx, pool, orgA, nil, "new", "form", nil, now)
	seedLeadFull(t, ctx, pool, orgA, nil, "new", "api", nil, now)
	var formLead, apiLead uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM leads WHERE organization_id = $1 AND source = 'form'`, orgA).Scan(&formLead); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM leads WHERE organization_id = $1 AND source = 'api'`, orgA).Scan(&apiLead); err != nil {
		t.Fatal(err)
	}
	seedTask(t, ctx, pool, orgA, formLead, taskSeed{assignee: andi, status: "open", dueAt: &past})
	seedTask(t, ctx, pool, orgA, apiLead, taskSeed{assignee: andi, status: "open", dueAt: &past})

	leadB := seedLeadAt(t, ctx, pool, orgB, nil, "new", now)
	seedTask(t, ctx, pool, orgB, leadB, taskSeed{assignee: memberB, status: "open", dueAt: &past})

	a := tenant.Context{OrganizationID: orgA}

	out, err := repo.Tasks(ctx, a, metrics.Filter{Source: ptr("form")})
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}
	for _, m := range out {
		if m.FullName == "Andi" && m.OverdueCount != 1 {
			t.Errorf("source=form: expected Andi overdue=1 (the api lead's task excluded), got %d", m.OverdueCount)
		}
	}

	out, err = repo.Tasks(ctx, a, metrics.Filter{Assignee: &andi})
	if err != nil || len(out) != 1 || out[0].MembershipID != andi || out[0].OverdueCount != 2 {
		t.Errorf("assignee filter: expected only Andi with overdue=2, got %+v (err %v)", out, err)
	}

	out, err = repo.Tasks(ctx, a, metrics.Filter{})
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("expected org A's two members only, got %d — leaked another org's members", len(out))
	}

	out, err = repo.Tasks(ctx, a, metrics.Filter{Assignee: &memberB})
	if err != nil || len(out) != 0 {
		t.Errorf("a foreign membership id must narrow to nothing, got %+v (err %v)", out, err)
	}

	rt, err := repo.ResponseTimes(ctx, a, metrics.Filter{})
	if err != nil {
		t.Fatalf("response times: %v", err)
	}
	total := 0
	for _, b := range rt.Buckets {
		total += b.Count
	}
	if total != 2 {
		t.Errorf("response times must count org A's 2 leads only, got %d", total)
	}
}
