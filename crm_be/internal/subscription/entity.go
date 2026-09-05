// Package subscription holds the minimal Phase 1 shape — a row proving
// every organization has a plan from the moment it registers. The full
// plan/entitlement/usage machinery is Phase 8; nothing here enforces any
// limit.
package subscription

import (
	"time"

	"github.com/google/uuid"
)

type Subscription struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	PlanCode           string
	Status             string
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	ExternalReference  *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Plan is what ResolvePlan hands back to a caller — the organization's
// plan code plus the capability answer already resolved for every
// member of Channels (plan.go). Code is for display only (kriteria #6
// in prd — "code tetap dikirim untuk ditampilkan, bukan untuk
// diputuskan"); a caller must never branch on it.
type Plan struct {
	Code     string
	Channels map[Channel]bool
	Limits   Limits
}
