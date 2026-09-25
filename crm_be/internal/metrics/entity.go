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

// Filter scopes every endpoint to leads.created_at — never any other
// timestamp (TD §2.1): "lead masuk periode ini, apa yang terjadi
// padanya". Both bounds are inclusive and optional; a nil bound is
// unbounded on that side (except /trend, which requires both — see
// Usecase.Trend).
//
// Assignee and Source (Phase 8.6 TD §2.1) only ever NARROW within the
// caller's organization: every query already pins organization_id from
// the tenant, so a membership id from another tenant matches nothing —
// an empty result, never a 403 that would confirm it exists.
type Filter struct {
	From     *time.Time
	To       *time.Time
	Assignee *uuid.UUID
	Source   *string
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

// TrendBucket is the width of one point on the lead-intake trend (Phase 8.6
// TD §2.2). Chosen by the server from the range, never by the client.
type TrendBucket string

const (
	TrendBucketDay  TrendBucket = "day"
	TrendBucketWeek TrendBucket = "week"
)

// TrendPoint is one bucket. Date is a CALENDAR date in the organization's
// timezone (Aturan #13), not an instant — it is sent as YYYY-MM-DD, and
// only its year/month/day are meaningful.
type TrendPoint struct {
	Date  time.Time
	Count int
}

// Trend is GET /v1/metrics/trend's payload: every bucket in the range,
// including empty ones — a line that skips days without leads lies.
type Trend struct {
	Bucket TrendBucket
	Points []TrendPoint
}

// SourceMetric is one row of GET /v1/metrics/sources — always four rows,
// zeros included (TD §2.3). ConversionRate uses exactly the /summary
// definition (won ÷ (total − spam − unqualified)), nil when that
// denominator is zero.
type SourceMetric struct {
	Source         string
	Count          int
	WonCount       int
	ConversionRate *float64
}

// LostReasonMetric is one row of GET /v1/metrics/lost-reasons — always six
// rows, zeros included (TD §2.4).
type LostReasonMetric struct {
	Reason string
	Count  int
}

// conversionRate is the ONE definition of conversion rate in this package
// (Phase 3 TD §2.2): spam and unqualified leave the DENOMINATOR. /summary
// and /sources both go through it, so the two numbers on the Laporan
// screen can't disagree about what the word means.
func conversionRate(total, won, spam, unqualified int) *float64 {
	denominator := total - spam - unqualified
	if denominator <= 0 {
		return nil
	}
	rate := float64(won) / float64(denominator)
	return &rate
}

// Response-time buckets (Phase 8.6 TD §2.5, brief §10.3). Boundaries are
// half-open, lower bound inclusive: exactly one hour is "1h_4h", exactly a
// day is "gt_24h". never_touched is its own bucket — a lead nobody has
// touched is not "> 1 hari", and folding it in would flatter a team that
// left it sitting.
const (
	ResponseLessThan1h   = "lt_1h"
	Response1hTo4h       = "1h_4h"
	Response4hTo24h      = "4h_24h"
	ResponseOver24h      = "gt_24h"
	ResponseNeverTouched = "never_touched"
)

type ResponseBucket struct {
	Bucket string
	Count  int
}

// ResponseTimes is GET /v1/metrics/response-times's payload. "Touched"
// means exactly what avg_response_seconds means on /employees: the first
// activity that isn't lead_created (Phase 3 TD §2.3) — one definition of
// "response" in this package. MedianSeconds is nil when no lead in range
// was ever touched.
type ResponseTimes struct {
	Buckets       []ResponseBucket
	MedianSeconds *float64
}

// TaskMetric is one row of GET /v1/metrics/tasks — every active
// membership, any role (TD §2.6).
//
// CompletedCount follows the range (completed_at within from/to).
// OverdueCount does NOT: it is open tasks past due RIGHT NOW. The tasks
// table keeps no history of lateness, so "how many were late 30 days ago"
// can't be answered — the screen must say so (TD §4.4). This is the only
// metric whose range is not leads.created_at.
type TaskMetric struct {
	MembershipID   uuid.UUID
	FullName       string
	CompletedCount int
	OverdueCount   int
}
