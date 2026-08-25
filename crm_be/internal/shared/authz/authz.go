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
		ActionMetricsRead: true,
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
		ActionCustomerRead: true,
		// ActionCustomerUpdate/Delete/ActionLeadConvert deliberately
		// absent — Employee gets read-only, repo-restricted to
		// customers converted from leads assigned to them.
		// ActionMetricsRead deliberately absent — the dashboard isn't
		// Employee's tool (Phase 3 TD §2.4); Employee gets mobile in
		// Phase 5.
	},
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
