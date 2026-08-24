// Package notification is the delivery mechanism other domains (lead,
// eventually task) use to tell a member something happened. It is
// deliberately the one resource in this codebase where even Owner/Admin
// get no broader access than any other role — a notification belongs to
// exactly one recipient, full stop (TD §8). See
// docs/phases/02-crm-core/td.md §2, §8, §11.
package notification

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	RecipientMembershipID uuid.UUID
	Type                  string
	LeadID                *uuid.UUID
	TaskID                *uuid.UUID
	Title                 string
	Body                  *string
	ReadAt                *time.Time
	CreatedAt             time.Time
}
