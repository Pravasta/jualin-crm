// Package lead is CRM Core's domain entity — the shape every capture
// source (manual, api, form, webhook) produces. As of #20 this is a full
// domain package (entity/port/usecase/repository_postgres/handler_http),
// having started in #19 as a pure repository. See
// docs/phases/02-crm-core/td.md §1.1.
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

	RawPayload []byte
	// SourceAPIKeyID is set only for a lead created through the public
	// API (Phase 4 #47) — nil for every other source, including a
	// dashboard lead whose Source happens to equal "api" from a stale
	// integration record (freeze 8.4: leads.source accepted 'api' since
	// 0003, this column arrived later in 0005).
	SourceAPIKeyID *uuid.UUID
	// SourceFormID is SourceAPIKeyID's Phase 6 (#87) counterpart — set
	// only for a lead created through POST /v1/forms/{public_key}/submit,
	// nil for every other source. leads.source accepted 'form' since
	// 0003; this column arrived later in 0007 (#85), same staggered
	// pattern SourceAPIKeyID's own doc comment describes.
	SourceFormID   *uuid.UUID
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
	SourceAPIKeyID         *uuid.UUID
	SourceFormID           *uuid.UUID
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

// ErrIdempotencyKeyExists is Create's signal that the (organization,
// idempotency_key) pair already exists — detected via the
// uq_leads_org_idempotency unique-violation, never a SELECT-then-INSERT
// (that would race under exactly the concurrent-retry conditions
// idempotency keys exist to handle). The usecase reacts by looking up
// and returning the existing lead instead of erroring.
var ErrIdempotencyKeyExists = errors.New("lead: idempotency key already used")

// ErrAssigneeNotFound is Create's signal that
// CreateInput.AssignedToMembershipID doesn't reference a real
// membership in this organization — detected via the fk_leads_assignee
// foreign-key violation, same database-constraint-as-source-of-truth
// pattern as ErrIdempotencyKeyExists. Found during #20's own testing: a
// bad assignee id used to surface as a bare 500.
var ErrAssigneeNotFound = errors.New("lead: assignee not found")

// mainPath is TD §5's primary funnel — index order matters, it's what
// "maju satu langkah" / "mundur satu langkah" measure against.
var mainPath = []string{"new", "contacted", "qualified", "proposal", "won"}

const (
	StatusLost        = "lost"
	StatusUnqualified = "unqualified"
	StatusSpam        = "spam"
)

// ListFilter is FindAllByOrg's argument — every field is optional
// (zero value = don't filter on it), except AssignedToNone which is a
// separate flag from AssignedTo since "filter to unassigned leads" and
// "don't filter on assignment" are different requests that both leave
// AssignedTo nil.
type ListFilter struct {
	Status         []string
	Source         []string
	AssignedTo     *uuid.UUID
	AssignedToNone bool
	Query          string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	Page           int
	PerPage        int
}

// CreateLeadInput is Usecase.Create's argument — pre-normalization
// (unlike port.go's CreateInput, which is what Repository.Create
// persists once the usecase has already validated/normalized
// everything). Deliberately distinct types: the repository layer must
// never need to know what "not yet validated" looks like.
type CreateLeadInput struct {
	Name                   string
	Email                  *string
	Phone                  *string
	Company                *string
	Notes                  *string
	Source                 string
	AssignedToMembershipID *uuid.UUID
	IdempotencyKey         *string
	// RawPayload is the request body exactly as received — populated
	// only by the public API handler (Phase 4 #47, TD §5), nil for the
	// dashboard's structured-field create. Deliberately NOT paired with
	// a SourceAPIKeyID field here even though TD §5's literal delta list
	// names one: source_api_key_id is derived by Usecase.Create directly
	// from t.APIKeyID (the already-authenticated principal), never from
	// caller input — the same reasoning Rule #5 applies to
	// organization_id. A field here would be a value nobody should ever
	// set except the usecase itself, which is exactly the shape of bug
	// Rule #5 exists to prevent. Documented as a deliberate TD deviation
	// in notes.md's "## #47" section. The identical reasoning applies to
	// source_form_id (Phase 6 #87): derived from t.FormID, never a field
	// here either.
	RawPayload []byte
}

// UpdateLeadInput is Usecase.Update's argument — same
// pre-normalization relationship to port.go's UpdateInput that
// CreateLeadInput has to CreateInput.
type UpdateLeadInput struct {
	Version int
	Name    *string
	Email   *string
	Phone   *string
	Company *string
	Notes   *string
}

// UpdateStatusInput is Usecase.UpdateStatus's argument.
type UpdateStatusInput struct {
	Version    int
	Status     string
	LostReason *string
}

// UpdateAssignmentInput is Usecase.UpdateAssignment's argument.
// AssignedToMembershipID nil means unassign — unlike UpdateLeadInput's
// fields, this one distinguishes "clear it" from "don't touch" because
// there's no third state to confuse it with (TD §8:
// {version, assigned_to_membership_id | null}, always present).
type UpdateAssignmentInput struct {
	Version                int
	AssignedToMembershipID *uuid.UUID
}

// VersionConflictError carries the row's current state — TD §4's 409
// body requirement ("body memuat keadaan terkini"). Same pattern as
// auth.OrganizationSelectionError: a documented, narrow extension of the
// error envelope for the one case that genuinely needs it, not license
// for arbitrary error payloads.
type VersionConflictError struct {
	Current *Lead
}

func (e *VersionConflictError) Error() string { return "version conflict" }
