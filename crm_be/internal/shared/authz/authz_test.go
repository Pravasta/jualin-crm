package authz_test

import (
	"errors"
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func TestRequire(t *testing.T) {
	cases := []struct {
		role    tenant.Role
		action  authz.Action
		allowed bool
	}{
		{tenant.RoleOwner, authz.ActionMembershipList, true},
		{tenant.RoleOwner, authz.ActionMembershipUpdateRole, true},
		{tenant.RoleOwner, authz.ActionMembershipDeactivate, true},
		{tenant.RoleOwner, authz.ActionInvitationCreate, true},
		{tenant.RoleOwner, authz.ActionInvitationList, true},
		{tenant.RoleOwner, authz.ActionInvitationRevoke, true},
		{tenant.RoleOwner, authz.ActionLeadCreate, true},
		{tenant.RoleOwner, authz.ActionLeadRead, true},
		{tenant.RoleOwner, authz.ActionLeadUpdate, true},
		{tenant.RoleOwner, authz.ActionLeadDelete, true},
		{tenant.RoleOwner, authz.ActionActivityCreate, true},
		{tenant.RoleOwner, authz.ActionActivityList, true},
		{tenant.RoleOwner, authz.ActionTaskCreate, true},
		{tenant.RoleOwner, authz.ActionTaskRead, true},
		{tenant.RoleOwner, authz.ActionTaskUpdate, true},
		{tenant.RoleOwner, authz.ActionTaskComplete, true},
		{tenant.RoleOwner, authz.ActionTaskDelete, true},

		{tenant.RoleAdmin, authz.ActionMembershipList, true},
		{tenant.RoleAdmin, authz.ActionMembershipUpdateRole, true},
		{tenant.RoleAdmin, authz.ActionMembershipDeactivate, true},
		{tenant.RoleAdmin, authz.ActionInvitationCreate, true},
		{tenant.RoleAdmin, authz.ActionInvitationList, true},
		{tenant.RoleAdmin, authz.ActionInvitationRevoke, true},
		{tenant.RoleAdmin, authz.ActionLeadCreate, true},
		{tenant.RoleAdmin, authz.ActionLeadRead, true},
		{tenant.RoleAdmin, authz.ActionLeadUpdate, true},
		{tenant.RoleAdmin, authz.ActionLeadDelete, true},
		{tenant.RoleAdmin, authz.ActionActivityCreate, true},
		{tenant.RoleAdmin, authz.ActionActivityList, true},
		{tenant.RoleAdmin, authz.ActionTaskCreate, true},
		{tenant.RoleAdmin, authz.ActionTaskRead, true},
		{tenant.RoleAdmin, authz.ActionTaskUpdate, true},
		{tenant.RoleAdmin, authz.ActionTaskComplete, true},
		{tenant.RoleAdmin, authz.ActionTaskDelete, true},

		{tenant.RoleManager, authz.ActionMembershipList, true},
		{tenant.RoleManager, authz.ActionMembershipUpdateRole, false},
		{tenant.RoleManager, authz.ActionMembershipDeactivate, false},
		{tenant.RoleManager, authz.ActionInvitationCreate, false},
		{tenant.RoleManager, authz.ActionInvitationList, false},
		{tenant.RoleManager, authz.ActionInvitationRevoke, false},
		{tenant.RoleManager, authz.ActionLeadCreate, true},
		{tenant.RoleManager, authz.ActionLeadRead, true},
		{tenant.RoleManager, authz.ActionLeadUpdate, true},
		{tenant.RoleManager, authz.ActionLeadDelete, false},
		{tenant.RoleManager, authz.ActionActivityCreate, true},
		{tenant.RoleManager, authz.ActionActivityList, true},
		{tenant.RoleManager, authz.ActionTaskCreate, true},
		{tenant.RoleManager, authz.ActionTaskRead, true},
		{tenant.RoleManager, authz.ActionTaskUpdate, true},
		{tenant.RoleManager, authz.ActionTaskComplete, true},
		{tenant.RoleManager, authz.ActionTaskDelete, true},

		{tenant.RoleEmployee, authz.ActionMembershipList, false},
		{tenant.RoleEmployee, authz.ActionMembershipUpdateRole, false},
		{tenant.RoleEmployee, authz.ActionMembershipDeactivate, false},
		{tenant.RoleEmployee, authz.ActionInvitationCreate, false},
		{tenant.RoleEmployee, authz.ActionInvitationList, false},
		{tenant.RoleEmployee, authz.ActionInvitationRevoke, false},
		{tenant.RoleEmployee, authz.ActionLeadCreate, false},
		{tenant.RoleEmployee, authz.ActionLeadRead, true}, // repository further restricts to own leads
		{tenant.RoleEmployee, authz.ActionLeadUpdate, true},
		{tenant.RoleEmployee, authz.ActionLeadDelete, false},
		{tenant.RoleEmployee, authz.ActionActivityCreate, true}, // repository further restricts to own leads
		{tenant.RoleEmployee, authz.ActionActivityList, true},
		{tenant.RoleEmployee, authz.ActionTaskCreate, true},
		{tenant.RoleEmployee, authz.ActionTaskRead, true},
		{tenant.RoleEmployee, authz.ActionTaskUpdate, true},
		{tenant.RoleEmployee, authz.ActionTaskComplete, true},
		{tenant.RoleEmployee, authz.ActionTaskDelete, false}, // the one action Employee never gets
	}

	for _, c := range cases {
		t.Run(string(c.role)+"/"+string(c.action), func(t *testing.T) {
			err := authz.Require(tenant.Context{Role: c.role}, c.action)
			if c.allowed && err != nil {
				t.Errorf("expected %s to be allowed for %s, got: %v", c.action, c.role, err)
			}
			if !c.allowed {
				var derr *httpx.DomainError
				if !errors.As(err, &derr) || derr.Code != "forbidden" {
					t.Errorf("expected forbidden DomainError for %s/%s, got: %v", c.role, c.action, err)
				}
			}
		})
	}
}
