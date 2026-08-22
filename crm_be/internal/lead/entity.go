// Package lead is CRM Core's domain entity — the shape every capture
// source (manual, api, form, webhook) produces. This issue (#19) ships
// only entity.go + repository_postgres.go: no usecase, no HTTP, same
// shape internal/membership had before #11 gave it a real consumer.
// See docs/phases/02-crm-core/td.md §1.1.
package lead

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Lead struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	LeadNumber     int

	Name      string
	Email     *string
	Phone     *string
	PhoneE164 *string
	Company   *string
	Notes     *string

	Status                 string
	LostReason             *string
	Source                 string
	AssignedToMembershipID *uuid.UUID

	RawPayload     []byte
	IdempotencyKey *string
	Version        int

	CreatedByMembershipID *uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

// CreateInput is what Repository.Create persists — no validation here.
// Field validation, status defaulting, and E.164 normalization belong to
// the usecase issue #20 adds; this package only writes what it's given.
type CreateInput struct {
	Name                   string
	Email                  *string
	Phone                  *string
	PhoneE164              *string
	Company                *string
	Notes                  *string
	Source                 string
	AssignedToMembershipID *uuid.UUID
	CreatedByMembershipID  *uuid.UUID
	RawPayload             []byte
	IdempotencyKey         *string
}

// UpdateInput is the mutable-field subset PATCH /v1/leads/{id} will
// expose (issue #20) — nil means "don't touch". Deliberately does not
// cover status or assignment: those have their own transition rules and
// land with #20/#22, not here.
type UpdateInput struct {
	Name    *string
	Email   *string
	Phone   *string
	Company *string
	Notes   *string
}

// ErrVersionConflict is what Update returns — alongside the row's
// CURRENT state — when expectedVersion no longer matches. Returning the
// current state here means the future usecase doesn't need a second
// query to build the 409 body TD §4 requires ("body memuat keadaan
// terkini").
var ErrVersionConflict = errors.New("lead: version conflict")
