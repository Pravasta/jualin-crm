// Package invitation implements TD phase 1 §6.1 (B4): inviting someone
// into an organization and the two-branch acceptance flow. The second
// branch (email already has an account) is the security-critical one —
// see Usecase.Accept's doc comment.
package invitation

import (
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// invitationTTL matches TD phase 1 §1's "kedaluwarsa 7 hari".
const invitationTTL = 7 * 24 * time.Hour

type Invitation struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	Email                 string
	Role                  tenant.Role
	TokenHash             string
	InvitedByMembershipID uuid.UUID
	ExpiresAt             time.Time
	AcceptedAt            *time.Time
	RevokedAt             *time.Time
	CreatedAt             time.Time
}

// CreateInput is Usecase.Create's argument.
type CreateInput struct {
	Email string
	Role  tenant.Role
}

// AcceptNewUserInput is the branch-1 body (TD §6.1): the email has no
// account yet, so this is also where the account is created. There is
// deliberately no equivalent field on AcceptExistingUserInput — see
// Usecase.Accept.
type AcceptNewUserInput struct {
	Token    string
	FullName string
	Password string
}

// AcceptExistingUserInput is the branch-2 body: the email already has an
// account. It carries ONLY the token — no Password field exists on this
// type, so there is no code path in that branch that could read or set
// one. This is the freeze bagian 7 (B4) security requirement enforced by
// the type system, not just a runtime check.
type AcceptExistingUserInput struct {
	Token string
}

// TokenInfo is GET /v1/invitations/token/{token}'s public payload.
type TokenInfo struct {
	OrganizationName string
	Email            string
	UserExists       bool
}
