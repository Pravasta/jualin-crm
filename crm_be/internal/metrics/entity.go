// Package metrics reads aggregates across leads/customers/activities for
// the owner dashboard home screen. It belongs to none of those three
// domains, so it gets its own folder (ADR-011) rather than being tacked
// onto internal/lead.
//
// This package is deliberately read-only — no Store/InTx, no Repos
// (Phase 3 TD §2). Adding a Unit of Work for a package that never writes
// would be unused machinery (Rule #27).
package metrics

import (
	"time"

	"github.com/google/uuid"
)

// Filter scopes both endpoints to leads.created_at — never any other
// timestamp (TD §2.1): "lead masuk periode ini, apa yang terjadi
// padanya". Both bounds are inclusive and optional; a nil bound is
// unbounded on that side.
type Filter struct {
	From *time.Time
	To   *time.Time
}

// Summary is GET /v1/metrics/summary's payload (TD §2.1).
type Summary struct {
	TotalNew   int
	ByStatus   map[string]int
	Unassigned int
	// ConversionRate is nil when the denominator (total minus spam minus
	// unqualified) is zero — "belum ada yang bisa dihitung" is not the
	// same as "sudah dicoba, gagal" (TD §2.2), so it is never reported
	// as a bare 0.
	ConversionRate *float64
}

// EmployeeMetric is one row of GET /v1/metrics/employees's payload (TD
// §2.1) — keyed by membership, not restricted to role=employee: any
// role can hold a lead assignment.
type EmployeeMetric struct {
	MembershipID uuid.UUID
	FullName     string
	LeadCount    int
	// AvgResponseSeconds is nil when this member has no assigned lead in
	// range that has ever been touched by an activity — excluded from
	// the average, not counted as zero (TD §2.3).
	AvgResponseSeconds *float64
	ConvertedCount     int
}
