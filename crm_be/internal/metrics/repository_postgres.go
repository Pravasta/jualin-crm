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

// leadRangeConditions builds the "leads.created_at within [from, to]"
// predicate shared by every query below — the ONLY timestamp Phase 3 TD
// §2.1 scopes either endpoint to. args starts with organization_id
// already in place ($1); each added bound gets its own placeholder via
// arg, same closure-based query-building pattern as
// internal/customer.postgresRepository.FindAllByOrg.
func leadRangeConditions(filter Filter, args *[]any, alias string) []string {
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
	return conditions
}

// Summary computes the four fields TD §2.1 defines, all scoped to
// leads.created_at within filter and to a single organization — no
// isEmployee branch (TD §2.4): Usecase never lets an Employee reach
// this method.
func (r *postgresRepository) Summary(ctx context.Context, t tenant.Context, filter Filter) (*Summary, error) {
	args := []any{t.OrganizationID}
	conditions := append([]string{"organization_id = $1", "deleted_at IS NULL"}, leadRangeConditions(filter, &args, "")...)
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
	// acceptance criterion #5.
	denominator := total - byStatus["spam"] - byStatus["unqualified"]
	var conversionRate *float64
	if denominator > 0 {
		rate := float64(byStatus["won"]) / float64(denominator)
		conversionRate = &rate
	}

	return &Summary{
		TotalNew:       total,
		ByStatus:       byStatus,
		Unassigned:     unassigned,
		ConversionRate: conversionRate,
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
	}, leadRangeConditions(filter, &args, "l.")...)

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
		WHERE m.organization_id = $1 AND m.deleted_at IS NULL
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
