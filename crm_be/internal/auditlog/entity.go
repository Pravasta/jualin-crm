// Package auditlog records sensitive actions. Entries are append-only —
// no update or delete path exists anywhere in this codebase, and none
// should ever be added.
package auditlog

import (
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type Entry struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	ActorType         tenant.PrincipalType
	ActorMembershipID *uuid.UUID
	Action            string
	EntityType        *string
	EntityID          *uuid.UUID
	RequestID         *string
	CreatedAt         time.Time
}
