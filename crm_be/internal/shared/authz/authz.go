// Package authz is the coarse "can this role perform this class of
// action at all" gate — enum action + a single function, per TD phase 1
// §9, backed by a Go map rather than a database permission table (freeze
// 1.3: "Role adalah enum. RBAC dinamis belum dibutuhkan").
//
// Relationship rules that depend on actor-vs-target rather than just the
// actor's role — "tidak ada yang bisa mengubah role dirinya sendiri",
// "owner terakhir tidak bisa menghapus dirinya sendiri", "admin tidak
// bisa menyentuh Owner" — are NOT expressible here and stay as explicit
// checks inside the usecase that owns the resource
// (internal/membership.Usecase). This package only answers the coarse
// question; see docs/architecture/authorization.md for the full matrix
// and the four numbered rules.
package authz

import (
	"net/http"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Action identifies one class of operation a role either can or cannot
// perform. New domains add their own constants here as they gain
// protected endpoints — this file is the single place the matrix lives
// in code.
type Action string

const (
	ActionMembershipList       Action = "membership.list"
	ActionMembershipUpdateRole Action = "membership.update_role"
	ActionMembershipDeactivate Action = "membership.deactivate"
	ActionInvitationCreate     Action = "invitation.create"
	ActionInvitationList       Action = "invitation.list"
	ActionInvitationRevoke     Action = "invitation.revoke"
)

// permissions mirrors docs/architecture/authorization.md's matrix
// exactly — Owner and Admin get every action Phase 1 defines; Manager
// gets read-only membership visibility; Employee gets none of it.
var permissions = map[tenant.Role]map[Action]bool{
	tenant.RoleOwner: {
		ActionMembershipList:       true,
		ActionMembershipUpdateRole: true,
		ActionMembershipDeactivate: true,
		ActionInvitationCreate:     true,
		ActionInvitationList:       true,
		ActionInvitationRevoke:     true,
	},
	tenant.RoleAdmin: {
		ActionMembershipList:       true,
		ActionMembershipUpdateRole: true,
		ActionMembershipDeactivate: true,
		ActionInvitationCreate:     true,
		ActionInvitationList:       true,
		ActionInvitationRevoke:     true,
	},
	tenant.RoleManager: {
		ActionMembershipList: true,
	},
	tenant.RoleEmployee: {},
}

// Require reports whether t.Role may perform action, returning a
// generic 403 forbidden (already catalogued in api.md) when it may not.
func Require(t tenant.Context, action Action) error {
	if permissions[t.Role][action] {
		return nil
	}
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "forbidden",
		Message: "Role Anda tidak mengizinkan aksi ini.",
	}
}
