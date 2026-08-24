// Package activity is CRM Core's append-only timeline. Rows are never
// updated or deleted (TD §1.3) — the absence of updated_at/deleted_at
// columns and of any PATCH/DELETE route is the enforcement, not a
// convention that has to be remembered elsewhere. See
// docs/phases/02-crm-core/td.md §1.3, §8, §10.
package activity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Activity struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	LeadID            uuid.UUID
	Type              string
	ActorMembershipID *uuid.UUID
	Body              *string
	// Metadata is raw jsonb bytes, never decoded back into a Go value —
	// its shape differs per Type (status_changed stores {from,to},
	// call_logged stores a duration, …) and TD §1.3 is explicit that it
	// is "tidak pernah di-query, hanya dirender di timeline". Callers
	// that need to render it just re-marshal this into the JSON
	// response verbatim.
	Metadata  json.RawMessage
	CreatedAt time.Time
}

// CreateInput is Repository.Create's argument. Metadata is a friendly
// map here — the repository is what converts it to jsonb bytes on the
// way in, keeping "how metadata is encoded" a single decision made in
// one place.
type CreateInput struct {
	LeadID            uuid.UUID
	Type              string
	ActorMembershipID *uuid.UUID
	Body              *string
	Metadata          map[string]any
}

// userTypes are the only activity types POST /v1/leads/{id}/activities
// accepts from a client (TD §8) — every other value in
// ck_activities_type is system-generated (TD §10) and gets 422 if a
// client tries to submit it. Allowing a client to fabricate a system
// type would make the timeline stop being trustworthy.
var userTypes = map[string]bool{
	"note_added":      true,
	"call_logged":     true,
	"whatsapp_opened": true,
}

// IsUserType reports whether t is one of the types a client may submit
// directly via POST.
func IsUserType(t string) bool {
	return userTypes[t]
}
