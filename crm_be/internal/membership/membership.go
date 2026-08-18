// Package membership is the tenant-scoped anchor domain — every composite
// FK elsewhere in this database points back to memberships(id,
// organization_id). See docs/architecture/multi-tenancy.md lapis 2.
package membership

import (
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Membership is a user's role within one organization. This is the
// entity ADR-003 refers to as "employee" — there is no separate employees
// table.
type Membership struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Role           tenant.Role
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}
