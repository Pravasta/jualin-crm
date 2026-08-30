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
	"slices"

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

	// Lead actions — list/read share one action (identical permissions
	// at every role); lead.convert lands with #23. Employee's read/
	// update access is further restricted to leads assigned to them,
	// enforced in internal/lead's repository (Rule: authz answers "this
	// class of action at all", not "which rows").
	ActionLeadCreate Action = "lead.create"
	ActionLeadRead   Action = "lead.read"
	ActionLeadUpdate Action = "lead.update"
	ActionLeadDelete Action = "lead.delete"
	// ActionLeadAssign is Owner/Admin/Manager only, same shape as
	// ActionLeadDelete — TD §9's matrix denies Employee both.
	ActionLeadAssign Action = "lead.assign"
	// ActionLeadConvert is Owner/Admin only — narrower than
	// ActionLeadDelete, which Manager also lacks but Employee's
	// exclusion is shared.
	ActionLeadConvert Action = "lead.convert"

	// Activity and task actions cover only what issue #21 ships.
	// Employee access is further restricted in the repository to leads
	// assigned to them (and, for task, to tasks on those leads) — same
	// split as lead's own read/update.
	ActionActivityCreate Action = "activity.create"
	ActionActivityList   Action = "activity.list"
	ActionTaskCreate     Action = "task.create"
	// ActionTaskRead has no dedicated row in TD §9's matrix — it's added
	// here with the same permissions as task.create/update/complete
	// (all four roles) so GET /v1/tasks and GET /v1/leads/{id}/tasks
	// have an explicit gate instead of reusing ActionTaskCreate for
	// reads, mirroring activity.list existing alongside activity.create.
	ActionTaskRead     Action = "task.read"
	ActionTaskUpdate   Action = "task.update"
	ActionTaskComplete Action = "task.complete"
	// ActionTaskDelete is the one action in this issue Employee does
	// NOT get, even repo-restricted — TD §9's matrix draws that line
	// explicitly.
	ActionTaskDelete Action = "task.delete"

	// Customer actions cover only what issue #23 ships. ActionCustomerRead
	// mirrors ActionLeadRead's shape (list+get share one action, Employee
	// repo-restricted through the originating lead's assignment).
	// ActionCustomerUpdate/Delete are Owner/Admin only — notably
	// narrower than lead.update/delete, which Manager also has: TD §9's
	// matrix draws this line deliberately, post-conversion editing is
	// more restricted than pre-conversion.
	ActionCustomerRead   Action = "customer.read"
	ActionCustomerUpdate Action = "customer.update"
	ActionCustomerDelete Action = "customer.delete"

	// ActionMetricsRead gates both aggregate endpoints issue #30 adds.
	// Employee is deliberately excluded — the dashboard isn't Employee's
	// tool (Employee gets mobile in Phase 5), and cross-organization
	// aggregates are management-level information regardless of role.
	ActionMetricsRead Action = "metrics.read"

	// ActionAPIKeyCreate/List/Revoke (Phase 4 #46) are Owner/Admin only —
	// narrower than membership.list, which Manager also has. A credential
	// that can inject leads into the organization, and the list of which
	// integrations are live, are not read-only information the way team
	// membership is; Manager and Employee get NO access at all, not
	// read-only. This map only answers "can PrincipalUser with this role
	// do this" — the separate question "what can PrincipalAPIKey itself
	// do" is #47's apiKeyScopeFor, which deliberately excludes all three
	// of these (Rule #24: an API key cannot create another API key).
	ActionAPIKeyCreate Action = "api_key.create"
	ActionAPIKeyList   Action = "api_key.list"   // #nosec G101 -- an RBAC action name, not a credential; gosec's identifier heuristic matches "APIKey" in the const name
	ActionAPIKeyRevoke Action = "api_key.revoke" // #nosec G101 -- same false positive as ActionAPIKeyList above

	// ActionDeviceTokenRegister/Delete (Phase 5 #68) are granted to
	// EVERY role, Owner through Employee — unlike api_key.*, which is
	// deliberately narrow. A device token is registering the CALLER'S
	// OWN phone for push, not a capability that reaches other people's
	// data; Owner and Admin use the dashboard today, but nothing stops
	// either of them from installing the mobile app too, and there's no
	// security reason to make them the exception.
	ActionDeviceTokenRegister Action = "device_token.register" // #nosec G101 -- an RBAC action name, not a credential; gosec's identifier heuristic matches "Token" in the const name
	ActionDeviceTokenDelete   Action = "device_token.delete"

	// ActionFormCreate/List/Read/Update/Delete (Phase 6 #85) are
	// Owner/Admin only — same shape and same reasoning as api_key.*
	// above: a form's public_key is a credential that can inject leads
	// into the organization (once #87 wires submission), so Manager and
	// Employee get no access at all, not read-only. The separate
	// question of what PrincipalPublicForm itself may do is #87's
	// publicFormAllows map — this map never grants that principal
	// anything, since it has no Role to look up here at all.
	ActionFormCreate Action = "form.create"
	ActionFormList   Action = "form.list"
	ActionFormRead   Action = "form.read"
	ActionFormUpdate Action = "form.update"
	ActionFormDelete Action = "form.delete"
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
		ActionLeadCreate:           true,
		ActionLeadRead:             true,
		ActionLeadUpdate:           true,
		ActionLeadDelete:           true,
		ActionLeadAssign:           true,
		ActionLeadConvert:          true,
		ActionActivityCreate:       true,
		ActionActivityList:         true,
		ActionTaskCreate:           true,
		ActionTaskRead:             true,
		ActionTaskUpdate:           true,
		ActionTaskComplete:         true,
		ActionTaskDelete:           true,
		ActionCustomerRead:         true,
		ActionCustomerUpdate:       true,
		ActionCustomerDelete:       true,
		ActionMetricsRead:          true,
		ActionAPIKeyCreate:         true,
		ActionAPIKeyList:           true,
		ActionAPIKeyRevoke:         true,
		ActionDeviceTokenRegister:  true,
		ActionDeviceTokenDelete:    true,
		ActionFormCreate:           true,
		ActionFormList:             true,
		ActionFormRead:             true,
		ActionFormUpdate:           true,
		ActionFormDelete:           true,
	},
	tenant.RoleAdmin: {
		ActionMembershipList:       true,
		ActionMembershipUpdateRole: true,
		ActionMembershipDeactivate: true,
		ActionInvitationCreate:     true,
		ActionInvitationList:       true,
		ActionInvitationRevoke:     true,
		ActionLeadCreate:           true,
		ActionLeadRead:             true,
		ActionLeadUpdate:           true,
		ActionLeadDelete:           true,
		ActionLeadAssign:           true,
		ActionLeadConvert:          true,
		ActionActivityCreate:       true,
		ActionActivityList:         true,
		ActionTaskCreate:           true,
		ActionTaskRead:             true,
		ActionTaskUpdate:           true,
		ActionTaskComplete:         true,
		ActionTaskDelete:           true,
		ActionCustomerRead:         true,
		ActionCustomerUpdate:       true,
		ActionCustomerDelete:       true,
		ActionMetricsRead:          true,
		ActionAPIKeyCreate:         true,
		ActionAPIKeyList:           true,
		ActionAPIKeyRevoke:         true,
		ActionDeviceTokenRegister:  true,
		ActionDeviceTokenDelete:    true,
		ActionFormCreate:           true,
		ActionFormList:             true,
		ActionFormRead:             true,
		ActionFormUpdate:           true,
		ActionFormDelete:           true,
	},
	tenant.RoleManager: {
		ActionMembershipList: true,
		ActionLeadCreate:     true,
		ActionLeadRead:       true,
		ActionLeadUpdate:     true,
		// ActionLeadDelete deliberately absent — matrix gives delete to
		// Owner/Admin only.
		ActionLeadAssign: true,
		// ActionLeadConvert deliberately absent — Owner/Admin only.
		ActionActivityCreate: true,
		ActionActivityList:   true,
		ActionTaskCreate:     true,
		ActionTaskRead:       true,
		ActionTaskUpdate:     true,
		ActionTaskComplete:   true,
		ActionTaskDelete:     true,
		ActionCustomerRead:   true,
		// ActionCustomerUpdate/Delete deliberately absent — Owner/Admin
		// only, narrower than Manager's lead.update/delete access.
		ActionMetricsRead:         true,
		ActionDeviceTokenRegister: true,
		ActionDeviceTokenDelete:   true,
	},
	tenant.RoleEmployee: {
		// Read/update granted here; the repository further restricts
		// both to leads assigned to this employee.
		ActionLeadRead:       true,
		ActionLeadUpdate:     true,
		ActionActivityCreate: true,
		ActionActivityList:   true,
		ActionTaskCreate:     true,
		ActionTaskRead:       true,
		ActionTaskUpdate:     true,
		ActionTaskComplete:   true,
		// ActionTaskDelete deliberately absent — TD §9's matrix denies
		// Employee this one, unlike every other activity/task action.
		ActionCustomerRead:        true,
		ActionDeviceTokenRegister: true,
		ActionDeviceTokenDelete:   true,
		// ActionCustomerUpdate/Delete/ActionLeadConvert deliberately
		// absent — Employee gets read-only, repo-restricted to
		// customers converted from leads assigned to them.
		// ActionMetricsRead deliberately absent — the dashboard isn't
		// Employee's tool (Phase 3 TD §2.4); Employee gets mobile in
		// Phase 5.
	},
}

// apiKeyScopeFor is the ONLY action principal api_key may ever perform,
// and the ONLY scope value that lets it (Phase 4 #47, TD §4 — keputusan
// D1). Every Action other than the ones listed here is denied because
// it is ABSENT from this map — not because someone remembered to write
// an exception for it. Rule #24 requires the penegakan live in a layer
// every caller passes through, not per handler: this map IS that layer.
// Extending what an API key can do means adding a row here, not
// auditing every handler in the codebase.
var apiKeyScopeFor = map[Action]string{
	ActionLeadCreate: "leads:write",
}

// Require reports whether t may perform action, returning a 403
// (already catalogued in api.md) when it may not. Two principals, two
// completely separate gates, chosen by t.PrincipalType — a
// PrincipalAPIKey context has no Role at all, so it must never fall
// through to permissions[""][action] and get an accidental answer
// either way; it is checked against apiKeyScopeFor and t.Scopes
// instead, and never touches the role-based map below.
func Require(t tenant.Context, action Action) error {
	if t.PrincipalType == tenant.PrincipalAPIKey {
		scope, ok := apiKeyScopeFor[action]
		if !ok || !slices.Contains(t.Scopes, scope) {
			return InsufficientScopeError()
		}
		return nil
	}
	if permissions[t.Role][action] {
		return nil
	}
	return forbiddenError()
}

func forbiddenError() error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "forbidden",
		Message: "Role Anda tidak mengizinkan aksi ini.",
	}
}

// InsufficientScopeError is exported because internal/lead needs the
// EXACT same code for one business rule Require itself never sees: an
// API key sending assigned_to_membership_id is syntactically valid but
// semantically impossible (no external system can know a membership
// id), so lead.Usecase.Create checks it directly rather than routing it
// through an Action. Reusing this constructor keeps that check and
// Require's own scope denial indistinguishable to a caller, which is
// the point — both are "this credential's scope doesn't cover this".
func InsufficientScopeError() error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "insufficient_scope",
		Message: "Kredensial ini tidak memiliki scope untuk aksi ini.",
	}
}
