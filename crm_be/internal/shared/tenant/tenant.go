// Package tenant defines the context every tenant-scoped repository
// method requires as its first parameter. See docs/architecture/multi-tenancy.md
// lapis 1 — a repository method that doesn't take a Context should never
// exist, so nothing can call it incorrectly.
package tenant

import "github.com/google/uuid"

// PrincipalType identifies what kind of caller a request came from.
// Checking OrganizationID alone is not enough (Rule #24): two requests
// with the same organization_id but a different PrincipalType are not
// equivalent requests.
type PrincipalType string

const (
	PrincipalUser       PrincipalType = "user"
	PrincipalAPIKey     PrincipalType = "api_key"
	PrincipalPublicForm PrincipalType = "public_form"
	PrincipalSystem     PrincipalType = "system"
)

// Role is a membership's role within an organization. It is a fixed enum,
// not a database-backed permission table — dynamic RBAC is out of scope
// until a customer actually asks for custom roles (freeze bagian 4.4).
type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleManager  Role = "manager"
	RoleEmployee Role = "employee"
)

// Context carries everything a tenant-scoped operation needs to know
// about who is making the request. organization_id always comes from an
// authenticated credential (a verified JWT claim, an API key lookup) —
// never from client-supplied body/query/header fields (Rule #5).
//
// Only PrincipalUser is populated in Phase 1. APIKeyID exists now so this
// struct's shape doesn't change when Phase 4 introduces API keys — the
// one piece of future-proofing this package allows itself, at the cost
// of a single nil field.
type Context struct {
	OrganizationID uuid.UUID
	PrincipalType  PrincipalType
	MembershipID   *uuid.UUID
	UserID         *uuid.UUID
	Role           Role
	APIKeyID       *uuid.UUID
	// Scopes is only ever populated when PrincipalType == PrincipalAPIKey
	// (Phase 4 #47, TD §4) — a role-based principal is gated by Role
	// against authz's permissions map; a scope-based principal has no
	// role at all and is gated by this slice against authz's separate
	// apiKeyScopeFor map instead. The two gates never mix.
	Scopes []string
	// FormID is only ever populated when PrincipalType ==
	// PrincipalPublicForm (Phase 6 #87) — same shape as APIKeyID before
	// it. It is the id of the credential (a forms.public_key row) that
	// resolved this request; lead.Usecase.Create reads it to stamp
	// leads.source_form_id — never from the request body (Rule #5).
	FormID    *uuid.UUID
	RequestID string
}
