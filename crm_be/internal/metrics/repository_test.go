package metrics_test

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

func TestRepository_Summary_ByStatusUnassignedAndRangeFilter(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)

	org := seedOrganization(t, ctx, pool)
	m := seedMembership(t, ctx, pool, org, "owner@example.com", "Owner Satu")

	inRange := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	outOfRange := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	seedLeadAt(t, ctx, pool, org, nil, "new", inRange)
	seedLeadAt(t, ctx, pool, org, &m, "contacted", inRange)
	seedLeadAt(t, ctx, pool, org, nil, "won", inRange)
	seedLeadAt(t, ctx, pool, org, nil, "new", outOfRange) // outside filter — must be excluded

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)
	s, err := repo.Summary(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{From: &from, To: &to})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	if s.TotalNew != 3 {
		t.Errorf("expected total_new=3 (out-of-range lead excluded), got %d", s.TotalNew)
	}
	if s.ByStatus["new"] != 1 || s.ByStatus["contacted"] != 1 || s.ByStatus["won"] != 1 {
		t.Errorf("unexpected by_status: %+v", s.ByStatus)
	}
	if s.Unassigned != 2 {
		t.Errorf("expected unassigned=2 (the contacted lead is assigned), got %d", s.Unassigned)
	}
}

// TestRepository_Summary_ConversionRate_ExcludesSpamAndUnqualifiedFromDenominator
// is the direct proof for TD §2.2 / issue #30's headline acceptance
// criterion: spam and unqualified must leave the DENOMINATOR, not just
// the numerator.
func TestRepository_Summary_ConversionRate_ExcludesSpamAndUnqualifiedFromDenominator(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)

	now := time.Now().UTC()
	seedLeadAt(t, ctx, pool, org, nil, "won", now)
	seedLeadAt(t, ctx, pool, org, nil, "new", now)
	seedLeadAt(t, ctx, pool, org, nil, "spam", now)
	seedLeadAt(t, ctx, pool, org, nil, "unqualified", now)

	s, err := repo.Summary(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	// total=4, denominator = 4 - 1(spam) - 1(unqualified) = 2, won=1 → 0.5.
	// If spam/unqualified were only excluded from the numerator, the
	// denominator would stay 4 and the rate would wrongly read 0.25.
	if s.ConversionRate == nil || *s.ConversionRate != 0.5 {
		t.Fatalf("expected conversion_rate=0.5, got %v", s.ConversionRate)
	}
}

func TestRepository_Summary_ConversionRate_NilWhenDenominatorZero(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)

	now := time.Now().UTC()
	seedLeadAt(t, ctx, pool, org, nil, "spam", now)
	seedLeadAt(t, ctx, pool, org, nil, "unqualified", now)

	s, err := repo.Summary(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if s.ConversionRate != nil {
		t.Errorf("expected nil conversion_rate when the denominator is zero (\"belum ada yang bisa dihitung\"), got %v", *s.ConversionRate)
	}
}

// TestRepository_Summary_ScopedToOrganization is the mandatory case TD
// §2.5 calls out explicitly: a leak here exposes another tenant's
// business shape, not just one row.
func TestRepository_Summary_ScopedToOrganization(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)

	now := time.Now().UTC()
	seedLeadAt(t, ctx, pool, orgA, nil, "new", now)
	seedLeadAt(t, ctx, pool, orgB, nil, "new", now)
	seedLeadAt(t, ctx, pool, orgB, nil, "new", now)

	s, err := repo.Summary(ctx, tenant.Context{OrganizationID: orgA}, metrics.Filter{})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if s.TotalNew != 1 {
		t.Fatalf("expected org A's summary to see only its own lead, got total_new=%d — leaked org B's data", s.TotalNew)
	}
}

func TestRepository_Employees_LeadCountAndConvertedCount(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	m1 := seedMembership(t, ctx, pool, org, "budi@example.com", "Budi")
	m2 := seedMembership(t, ctx, pool, org, "sari@example.com", "Sari")

	now := time.Now().UTC()
	won := seedLeadAt(t, ctx, pool, org, &m1, "won", now)
	seedLeadAt(t, ctx, pool, org, &m1, "new", now)
	seedLeadAt(t, ctx, pool, org, &m2, "new", now)
	seedCustomerFromLead(t, ctx, pool, org, won)

	out, err := repo.Employees(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{})
	if err != nil {
		t.Fatalf("employees: %v", err)
	}

	byID := map[uuid.UUID]*metrics.EmployeeMetric{}
	for _, em := range out {
		byID[em.MembershipID] = em
	}

	if byID[m1].LeadCount != 2 {
		t.Errorf("expected m1 lead_count=2, got %d", byID[m1].LeadCount)
	}
	if byID[m1].ConvertedCount != 1 {
		t.Errorf("expected m1 converted_count=1, got %d", byID[m1].ConvertedCount)
	}
	if byID[m2].LeadCount != 1 {
		t.Errorf("expected m2 lead_count=1, got %d", byID[m2].LeadCount)
	}
	if byID[m2].ConvertedCount != 0 {
		t.Errorf("expected m2 converted_count=0, got %d", byID[m2].ConvertedCount)
	}
}

// TestRepository_Employees_AvgResponseSeconds_ExcludesUntouchedLeadsAndLeadCreated
// proves both halves of TD §2.3 at once: a 'lead_created' activity must
// NOT count as a touch (it fires automatically on capture — TD §2.3's
// exact reason to exclude it), and a lead with no OTHER activity must be
// excluded from the average rather than pulling it toward zero.
func TestRepository_Employees_AvgResponseSeconds_ExcludesUntouchedLeadsAndLeadCreated(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	org := seedOrganization(t, ctx, pool)
	m := seedMembership(t, ctx, pool, org, "budi@example.com", "Budi")

	created := time.Now().UTC().Add(-1 * time.Hour)
	touchedLead := seedLeadAt(t, ctx, pool, org, &m, "contacted", created)
	seedLeadAt(t, ctx, pool, org, &m, "new", created) // never touched by a real activity

	// lead_created fires at the same instant as creation — if this were
	// counted, avg_response_seconds would read ~0 instead of ~10 minutes.
	seedActivity(t, ctx, pool, org, touchedLead, "lead_created", created)
	seedActivity(t, ctx, pool, org, touchedLead, "status_changed", created.Add(10*time.Minute))

	out, err := repo.Employees(ctx, tenant.Context{OrganizationID: org}, metrics.Filter{})
	if err != nil {
		t.Fatalf("employees: %v", err)
	}

	var got *metrics.EmployeeMetric
	for _, em := range out {
		if em.MembershipID == m {
			got = em
		}
	}
	if got == nil {
		t.Fatal("expected the seeded membership in the output")
	}
	if got.AvgResponseSeconds == nil {
		t.Fatal("expected avg_response_seconds to be set from the touched lead's status_changed activity")
	}
	want := (10 * time.Minute).Seconds()
	if diff := *got.AvgResponseSeconds - want; diff < -1 || diff > 1 {
		t.Errorf("expected avg_response_seconds ~= %v (lead_created excluded, untouched lead excluded not zeroed), got %v", want, *got.AvgResponseSeconds)
	}
}

func TestRepository_Employees_ScopedToOrganization(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := metrics.New(pool)
	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	seedMembership(t, ctx, pool, orgA, "a@example.com", "A")
	seedMembership(t, ctx, pool, orgB, "b@example.com", "B")

	out, err := repo.Employees(ctx, tenant.Context{OrganizationID: orgA}, metrics.Filter{})
	if err != nil {
		t.Fatalf("employees: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected exactly 1 membership visible for org A, got %d — leaked another org's members", len(out))
	}
}

// --- seed helpers — internal/metrics doesn't depend on internal/lead,
// internal/membership, internal/customer, or internal/activity, so its
// tests build the rows they need with raw SQL, same pattern
// internal/customer's tests already use for leads.

func seedOrganization(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Test Org')`, id); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	return id
}

func seedMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, email, fullName string) uuid.UUID {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', $3)`, userID, email, fullName); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	membershipID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'employee')`, membershipID, org, userID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return membershipID
}

func seedLeadAt(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, assignedTo *uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	var q string
	if status == "lost" {
		q = `INSERT INTO leads (id, organization_id, lead_number, name, source, assigned_to_membership_id, status, lost_reason, created_at)
			VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), 'Test Lead', 'manual', $3, $4, 'other', $5)`
	} else {
		q = `INSERT INTO leads (id, organization_id, lead_number, name, source, assigned_to_membership_id, status, created_at)
			VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), 'Test Lead', 'manual', $3, $4, $5)`
	}
	if _, err := pool.Exec(ctx, q, id, org, assignedTo, status, createdAt); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE organizations SET next_lead_number = next_lead_number + 1 WHERE id = $1`, org); err != nil {
		t.Fatalf("bump next_lead_number: %v", err)
	}
	return id
}

func seedActivity(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, leadID uuid.UUID, activityType string, createdAt time.Time) {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	const q = `INSERT INTO activities (id, organization_id, lead_id, type, created_at) VALUES ($1, $2, $3, $4, $5)`
	if _, err := pool.Exec(ctx, q, id, org, leadID, activityType, createdAt); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
}

func seedCustomerFromLead(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, leadID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	const q = `INSERT INTO customers (id, organization_id, name, converted_from_lead_id) VALUES ($1, $2, 'Test Customer', $3)`
	if _, err := pool.Exec(ctx, q, id, org, leadID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	return id
}
