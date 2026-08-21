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

// MemberWithUser is Membership plus the display fields GET /v1/memberships
// needs — a joined read model for Usecase.List, not a second source of
// truth: the join lives entirely in one query (FindAllByOrgWithUser)
// rather than an N+1 loop of individual user lookups per member.
type MemberWithUser struct {
	Membership
	Email    string
	FullName string
}

// UpdateRoleInput and the two rule violations below are Usecase.UpdateRole's
// vocabulary — kept here alongside Membership as this package's domain
// types, matching the auth package's convention of colocating usecase-level
// input types with entities.
type UpdateRoleInput struct {
	MembershipID uuid.UUID
	NewRole      tenant.Role
}
