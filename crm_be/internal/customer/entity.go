// Package customer is what a lead becomes after an explicit conversion
// (B9 — never automatic, freeze 2.4 aturan 5). Data is copied from the
// originating lead at conversion time, not referenced — editing a
// customer never changes the lead it came from. See
// docs/phases/02-crm-core/td.md §1.2, §12.
package customer

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Customer struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID

	Name      string
	Email     *string
	Phone     *string
	PhoneE164 *string
	Company   *string
	Notes     *string

	ConvertedFromLeadID     uuid.UUID
	ConvertedByMembershipID *uuid.UUID
	ConvertedAt             time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// UpdateInput is the mutable-field subset PATCH /v1/customers/{id}
// exposes — nil means "don't touch", same limitation as
// lead.UpdateInput. No Version field: customers aren't edited from
// offline mobile (TD §8), so there's no write conflict to detect.
type UpdateInput struct {
	Name    *string
	Email   *string
	Phone   *string
	Company *string
	Notes   *string
}

// ListFilter is FindAllByOrg's argument.
type ListFilter struct {
	Query   string
	Page    int
	PerPage int
}

// ErrLeadNotWon is Convert's signal that the lead exists (and is
// visible to the caller) but isn't status=won — the repository can't
// tell this apart from "lead doesn't exist" by row count alone, so it
// resolves the ambiguity itself and returns this distinct sentinel.
var ErrLeadNotWon = errors.New("customer: lead not won")

// ErrAlreadyConverted is Convert's signal that uq_customers_org_lead
// was violated — detected via the unique-violation, never a
// SELECT-then-INSERT, same database-constraint-as-source-of-truth
// pattern as lead.ErrIdempotencyKeyExists.
var ErrAlreadyConverted = errors.New("customer: lead already converted")
