package metrics

import (
	"context"
	"fmt"
	"strings"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type postgresRepository struct {
	q db.Querier
}

// New wires a postgresRepository against q — same pattern as
// internal/customer.New. Called directly with *pgxpool.Pool at the
// composition root; there is no InTx wrapper because this package never
// writes.
func New(q db.Querier) Repository {
	return &postgresRepository{q: q}
}

// leadFilterConditions builds the predicate every query here shares:
// leads.created_at within [from, to] — the ONLY timestamp Phase 3 TD §2.1
// scopes these endpoints to — plus the optional assignee/source narrowing
// Phase 8.6 TD §2.1 adds. args starts with organization_id already in
// place ($1); each added value gets its own placeholder via arg, same
// closure-based query-building pattern as
// internal/customer.postgresRepository.FindAllByOrg.
func leadFilterConditions(filter Filter, args *[]any, alias string) []string {
	conditions := []string{}
	arg := func(v any) string {
		*args = append(*args, v)
		return fmt.Sprintf("$%d", len(*args))
	}
	if filter.From != nil {
		conditions = append(conditions, alias+"created_at >= "+arg(*filter.From))
	}
	if filter.To != nil {
		conditions = append(conditions, alias+"created_at <= "+arg(*filter.To))
	}
	if filter.Assignee != nil {
		conditions = append(conditions, alias+"assigned_to_membership_id = "+arg(*filter.Assignee))
	}
	if filter.Source != nil {
		conditions = append(conditions, alias+"source = "+arg(*filter.Source))
	}
	return conditions
}

// Summary computes the four fields TD §2.1 defines, all scoped to
// leads.created_at within filter and to a single organization — no
// isEmployee branch (TD §2.4): Usecase never lets an Employee reach
// this method.
func (r *postgresRepository) Summary(ctx context.Context, t tenant.Context, filter Filter) (*Summary, error) {
	args := []any{t.OrganizationID}
	conditions := append([]string{"organization_id = $1", "deleted_at IS NULL"}, leadFilterConditions(filter, &args, "")...)
	where := strings.Join(conditions, " AND ")

	byStatusQ := "SELECT status, count(*) FROM leads WHERE " + where + " GROUP BY status"
	rows, err := r.q.Query(ctx, byStatusQ, args...)
	if err != nil {
		return nil, fmt.Errorf("metrics: summary by status: %w", err)
	}
	byStatus := map[string]int{}
	total := 0
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("metrics: scan by status: %w", err)
		}
		byStatus[status] = count
		total += count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metrics: summary by status: %w", err)
	}

	unassignedQ := "SELECT count(*) FROM leads WHERE " + where + " AND assigned_to_membership_id IS NULL"
	var unassigned int
	if err := r.q.QueryRow(ctx, unassignedQ, args...).Scan(&unassigned); err != nil {
		return nil, fmt.Errorf("metrics: summary unassigned: %w", err)
	}

	// spam/unqualified excluded from the DENOMINATOR, not just the
	// numerator (TD §2.2) — this is what finally enforces Phase 2's
	// acceptance criterion #5. One helper, shared with Sources.
	return &Summary{
		TotalNew:       total,
		ByStatus:       byStatus,
		Unassigned:     unassigned,
		ConversionRate: conversionRate(total, byStatus["won"], byStatus["spam"], byStatus["unqualified"]),
	}, nil
}

// Employees aggregates per membership — every active membership in the
// organization, not just role=employee (assignment isn't role-
// restricted). lead_count/converted_count/avg_response_seconds are all
// scoped to leads assigned to that membership with created_at inside
// filter (TD §2.1: "Rentang membatasi leads.created_at, bukan waktu
// peristiwa lain").
//
// avg_response_seconds is computed as
// MIN(activities.created_at WHERE type <> 'lead_created') -
// leads.created_at per lead (TD §2.3), then averaged in SQL — Postgres's
// avg() ignores NULL inputs on its own, which is exactly "a never-
// touched lead is excluded from the average, not counted as zero"
// without any special-casing in this query.
func (r *postgresRepository) Employees(ctx context.Context, t tenant.Context, filter Filter) ([]*EmployeeMetric, error) {
	args := []any{t.OrganizationID}
	leadJoin := append([]string{
		"l.assigned_to_membership_id = m.id",
		"l.organization_id = m.organization_id",
		"l.deleted_at IS NULL",
	}, leadFilterConditions(filter, &args, "l.")...)

	// Filtering by one member narrows the ROWS too, not just the leads
	// joined to them — otherwise every other member would still be listed,
	// all zeros, which reads as "they had nothing" rather than "not shown".
	assigneeRow := ""
	if filter.Assignee != nil {
		args = append(args, *filter.Assignee)
		assigneeRow = fmt.Sprintf(" AND m.id = $%d", len(args))
	}

	q := `
		SELECT
			m.id,
			u.full_name,
			count(l.id) AS lead_count,
			count(l.id) FILTER (WHERE c.id IS NOT NULL) AS converted_count,
			avg(extract(epoch FROM (touch.first_touched_at - l.created_at))) AS avg_response_seconds
		FROM memberships m
		JOIN users u ON u.id = m.user_id
		LEFT JOIN leads l ON ` + strings.Join(leadJoin, " AND ") + `
		LEFT JOIN customers c ON c.converted_from_lead_id = l.id AND c.organization_id = m.organization_id
		LEFT JOIN LATERAL (
			SELECT MIN(a.created_at) AS first_touched_at
			FROM activities a
			WHERE a.lead_id = l.id AND a.organization_id = m.organization_id AND a.type <> 'lead_created'
		) touch ON true
		WHERE m.organization_id = $1 AND m.deleted_at IS NULL` + assigneeRow + `
		GROUP BY m.id, u.full_name
		ORDER BY u.full_name`

	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("metrics: employees: %w", err)
	}
	defer rows.Close()

	var out []*EmployeeMetric
	for rows.Next() {
		var em EmployeeMetric
		if err := rows.Scan(&em.MembershipID, &em.FullName, &em.LeadCount, &em.ConvertedCount, &em.AvgResponseSeconds); err != nil {
			return nil, fmt.Errorf("metrics: scan employee: %w", err)
		}
		out = append(out, &em)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metrics: employees: %w", err)
	}
	return out, nil
}

// Trend buckets leads by CALENDAR day or week in organizations.timezone
// (Aturan #13, Phase 8.6 TD §2.2) — a lead at 23:30 WIT is that day in
// Jayapura even though it is 14:30 UTC. generate_series produces every
// bucket in the range so empty ones come back as 0 rather than missing.
// Weeks start on Monday (date_trunc('week')). filter.From/To must be set;
// Usecase.Trend guarantees it.
func (r *postgresRepository) Trend(ctx context.Context, t tenant.Context, filter Filter, bucket TrendBucket) (*Trend, error) {
	if filter.From == nil || filter.To == nil {
		return nil, fmt.Errorf("metrics: trend: range required")
	}
	args := []any{t.OrganizationID, string(bucket), *filter.From, *filter.To}
	join := append([]string{
		"l.organization_id = $1",
		"l.deleted_at IS NULL",
		"date_trunc($2, l.created_at AT TIME ZONE o.timezone) = b.bucket",
	}, leadFilterConditions(filter, &args, "l.")...)

	q := `
		WITH o AS (SELECT timezone FROM organizations WHERE id = $1),
		buckets AS (
			SELECT generate_series(
				date_trunc($2, $3::timestamptz AT TIME ZONE o.timezone),
				date_trunc($2, $4::timestamptz AT TIME ZONE o.timezone),
				('1 ' || $2)::interval
			) AS bucket
			FROM o
		)
		SELECT b.bucket::date, count(l.id)
		FROM buckets b
		CROSS JOIN o
		LEFT JOIN leads l ON ` + strings.Join(join, " AND ") + `
		GROUP BY b.bucket
		ORDER BY b.bucket`

	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("metrics: trend: %w", err)
	}
	defer rows.Close()

	out := &Trend{Bucket: bucket, Points: []TrendPoint{}}
	for rows.Next() {
		var p TrendPoint
		if err := rows.Scan(&p.Date, &p.Count); err != nil {
			return nil, fmt.Errorf("metrics: scan trend: %w", err)
		}
		out.Points = append(out.Points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metrics: trend: %w", err)
	}
	return out, nil
}

// leadSources and lostReasons mirror the CHECK constraints in
// 0003_crm_core.sql (ck_leads_source, ck_leads_lost_reason), in the order
// the screen lists them. Both queries unnest them so every value comes
// back, zeros included — a source nobody used is information, not noise.
var (
	leadSources = []string{"manual", "api", "form", "webhook"}
	lostReasons = []string{"price", "competitor", "timing", "no_response", "not_interested", "other"}
)

func (r *postgresRepository) Sources(ctx context.Context, t tenant.Context, filter Filter) ([]*SourceMetric, error) {
	args := []any{t.OrganizationID, leadSources}
	join := append([]string{
		"l.organization_id = $1",
		"l.deleted_at IS NULL",
		"l.source = s.source",
	}, leadFilterConditions(filter, &args, "l.")...)

	q := `
		SELECT
			s.source,
			count(l.id),
			count(l.id) FILTER (WHERE l.status = 'won'),
			count(l.id) FILTER (WHERE l.status = 'spam'),
			count(l.id) FILTER (WHERE l.status = 'unqualified')
		FROM unnest($2::text[]) WITH ORDINALITY AS s(source, ord)
		LEFT JOIN leads l ON ` + strings.Join(join, " AND ") + `
		GROUP BY s.source, s.ord
		ORDER BY s.ord`

	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("metrics: sources: %w", err)
	}
	defer rows.Close()

	out := []*SourceMetric{}
	for rows.Next() {
		var m SourceMetric
		var spam, unqualified int
		if err := rows.Scan(&m.Source, &m.Count, &m.WonCount, &spam, &unqualified); err != nil {
			return nil, fmt.Errorf("metrics: scan sources: %w", err)
		}
		m.ConversionRate = conversionRate(m.Count, m.WonCount, spam, unqualified)
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metrics: sources: %w", err)
	}
	return out, nil
}

// LostReasons counts only status = 'lost' — lost_reason is guaranteed
// non-null there by ck_leads_lost_requires_reason.
func (r *postgresRepository) LostReasons(ctx context.Context, t tenant.Context, filter Filter) ([]*LostReasonMetric, error) {
	args := []any{t.OrganizationID, lostReasons}
	join := append([]string{
		"l.organization_id = $1",
		"l.deleted_at IS NULL",
		"l.status = 'lost'",
		"l.lost_reason = r.reason",
	}, leadFilterConditions(filter, &args, "l.")...)

	q := `
		SELECT r.reason, count(l.id)
		FROM unnest($2::text[]) WITH ORDINALITY AS r(reason, ord)
		LEFT JOIN leads l ON ` + strings.Join(join, " AND ") + `
		GROUP BY r.reason, r.ord
		ORDER BY r.ord`

	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("metrics: lost reasons: %w", err)
	}
	defer rows.Close()

	out := []*LostReasonMetric{}
	for rows.Next() {
		var m LostReasonMetric
		if err := rows.Scan(&m.Reason, &m.Count); err != nil {
			return nil, fmt.Errorf("metrics: scan lost reasons: %w", err)
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metrics: lost reasons: %w", err)
	}
	return out, nil
}
