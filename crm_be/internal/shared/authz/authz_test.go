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
		{tenant.RoleOwner, authz.ActionLeadAssign, true},
		{tenant.RoleOwner, authz.ActionLeadConvert, true},
		{tenant.RoleOwner, authz.ActionActivityCreate, true},
		{tenant.RoleOwner, authz.ActionActivityList, true},
		{tenant.RoleOwner, authz.ActionTaskCreate, true},
		{tenant.RoleOwner, authz.ActionTaskRead, true},
		{tenant.RoleOwner, authz.ActionTaskUpdate, true},
		{tenant.RoleOwner, authz.ActionTaskComplete, true},
		{tenant.RoleOwner, authz.ActionTaskDelete, true},
		{tenant.RoleOwner, authz.ActionCustomerRead, true},
		{tenant.RoleOwner, authz.ActionCustomerUpdate, true},
		{tenant.RoleOwner, authz.ActionCustomerDelete, true},
		{tenant.RoleOwner, authz.ActionMetricsRead, true},
		{tenant.RoleOwner, authz.ActionAPIKeyCreate, true},
		{tenant.RoleOwner, authz.ActionAPIKeyList, true},
		{tenant.RoleOwner, authz.ActionAPIKeyRevoke, true},
		{tenant.RoleOwner, authz.ActionDeviceTokenRegister, true},
		{tenant.RoleOwner, authz.ActionDeviceTokenDelete, true},
		{tenant.RoleOwner, authz.ActionFormCreate, true},
		{tenant.RoleOwner, authz.ActionFormList, true},
		{tenant.RoleOwner, authz.ActionFormRead, true},
		{tenant.RoleOwner, authz.ActionFormUpdate, true},
		{tenant.RoleOwner, authz.ActionFormDelete, true},
		{tenant.RoleOwner, authz.ActionWebhookCreate, true},
		{tenant.RoleOwner, authz.ActionWebhookList, true},
		{tenant.RoleOwner, authz.ActionWebhookRead, true},
		{tenant.RoleOwner, authz.ActionWebhookUpdate, true},
		{tenant.RoleOwner, authz.ActionWebhookDelete, true},
		{tenant.RoleOwner, authz.ActionSubscriptionChange, true},

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
		{tenant.RoleAdmin, authz.ActionLeadAssign, true},
		{tenant.RoleAdmin, authz.ActionLeadConvert, true},
		{tenant.RoleAdmin, authz.ActionActivityCreate, true},
		{tenant.RoleAdmin, authz.ActionActivityList, true},
		{tenant.RoleAdmin, authz.ActionTaskCreate, true},
		{tenant.RoleAdmin, authz.ActionTaskRead, true},
		{tenant.RoleAdmin, authz.ActionTaskUpdate, true},
		{tenant.RoleAdmin, authz.ActionTaskComplete, true},
		{tenant.RoleAdmin, authz.ActionTaskDelete, true},
		{tenant.RoleAdmin, authz.ActionCustomerRead, true},
		{tenant.RoleAdmin, authz.ActionCustomerUpdate, true},
		{tenant.RoleAdmin, authz.ActionCustomerDelete, true},
		{tenant.RoleAdmin, authz.ActionMetricsRead, true},
		{tenant.RoleAdmin, authz.ActionAPIKeyCreate, true},
		{tenant.RoleAdmin, authz.ActionAPIKeyList, true},
		{tenant.RoleAdmin, authz.ActionAPIKeyRevoke, true},
		{tenant.RoleAdmin, authz.ActionDeviceTokenRegister, true},
		{tenant.RoleAdmin, authz.ActionDeviceTokenDelete, true},
		{tenant.RoleAdmin, authz.ActionFormCreate, true},
		{tenant.RoleAdmin, authz.ActionFormList, true},
		{tenant.RoleAdmin, authz.ActionFormRead, true},
		{tenant.RoleAdmin, authz.ActionFormUpdate, true},
		{tenant.RoleAdmin, authz.ActionFormDelete, true},
		{tenant.RoleAdmin, authz.ActionWebhookCreate, true},
		{tenant.RoleAdmin, authz.ActionWebhookList, true},
		{tenant.RoleAdmin, authz.ActionWebhookRead, true},
		{tenant.RoleAdmin, authz.ActionWebhookUpdate, true},
		{tenant.RoleAdmin, authz.ActionWebhookDelete, true},
		{tenant.RoleAdmin, authz.ActionSubscriptionChange, false}, // Owner-only — the first action Admin does NOT mirror Owner on

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
		{tenant.RoleManager, authz.ActionLeadAssign, true},
		{tenant.RoleManager, authz.ActionLeadConvert, false},
		{tenant.RoleManager, authz.ActionActivityCreate, true},
		{tenant.RoleManager, authz.ActionActivityList, true},
		{tenant.RoleManager, authz.ActionTaskCreate, true},
		{tenant.RoleManager, authz.ActionTaskRead, true},
		{tenant.RoleManager, authz.ActionTaskUpdate, true},
		{tenant.RoleManager, authz.ActionTaskComplete, true},
		{tenant.RoleManager, authz.ActionTaskDelete, true},
		{tenant.RoleManager, authz.ActionCustomerRead, true},
		{tenant.RoleManager, authz.ActionCustomerUpdate, false},
		{tenant.RoleManager, authz.ActionCustomerDelete, false},
		{tenant.RoleManager, authz.ActionMetricsRead, true},
		{tenant.RoleManager, authz.ActionAPIKeyCreate, false}, // Manager gets NO access at all, not read-only
		{tenant.RoleManager, authz.ActionAPIKeyList, false},
		{tenant.RoleManager, authz.ActionAPIKeyRevoke, false},
		{tenant.RoleManager, authz.ActionDeviceTokenRegister, true}, // unlike api_key.*, every role gets this — it's the CALLER's own device
		{tenant.RoleManager, authz.ActionDeviceTokenDelete, true},
		{tenant.RoleManager, authz.ActionFormCreate, false}, // Manager gets NO access at all, not read-only — same shape as api_key.*
		{tenant.RoleManager, authz.ActionFormList, false},
		{tenant.RoleManager, authz.ActionFormRead, false},
		{tenant.RoleManager, authz.ActionFormUpdate, false},
		{tenant.RoleManager, authz.ActionFormDelete, false},
		{tenant.RoleManager, authz.ActionWebhookCreate, false}, // Manager gets NO access at all — same shape as api_key.*/form.*
		{tenant.RoleManager, authz.ActionWebhookList, false},
		{tenant.RoleManager, authz.ActionWebhookRead, false},
		{tenant.RoleManager, authz.ActionWebhookUpdate, false},
		{tenant.RoleManager, authz.ActionWebhookDelete, false},
		{tenant.RoleManager, authz.ActionSubscriptionChange, false},

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
		{tenant.RoleEmployee, authz.ActionLeadAssign, false},
		{tenant.RoleEmployee, authz.ActionLeadConvert, false},
		{tenant.RoleEmployee, authz.ActionActivityCreate, true}, // repository further restricts to own leads
		{tenant.RoleEmployee, authz.ActionActivityList, true},
		{tenant.RoleEmployee, authz.ActionTaskCreate, true},
		{tenant.RoleEmployee, authz.ActionTaskRead, true},
		{tenant.RoleEmployee, authz.ActionTaskUpdate, true},
		{tenant.RoleEmployee, authz.ActionTaskComplete, true},
		{tenant.RoleEmployee, authz.ActionTaskDelete, false},  // the one action Employee never gets
		{tenant.RoleEmployee, authz.ActionCustomerRead, true}, // repository further restricts to own leads' customers
		{tenant.RoleEmployee, authz.ActionCustomerUpdate, false},
		{tenant.RoleEmployee, authz.ActionCustomerDelete, false},
		{tenant.RoleEmployee, authz.ActionMetricsRead, false}, // dashboard isn't Employee's tool
		{tenant.RoleEmployee, authz.ActionAPIKeyCreate, false},
		{tenant.RoleEmployee, authz.ActionAPIKeyList, false},
		{tenant.RoleEmployee, authz.ActionAPIKeyRevoke, false},
		{tenant.RoleEmployee, authz.ActionDeviceTokenRegister, true}, // every role, including Employee — registering a phone for push isn't a management action
		{tenant.RoleEmployee, authz.ActionDeviceTokenDelete, true},
		{tenant.RoleEmployee, authz.ActionFormCreate, false},
		{tenant.RoleEmployee, authz.ActionFormList, false},
		{tenant.RoleEmployee, authz.ActionFormRead, false},
		{tenant.RoleEmployee, authz.ActionFormUpdate, false},
		{tenant.RoleEmployee, authz.ActionFormDelete, false},
		{tenant.RoleEmployee, authz.ActionWebhookCreate, false},
		{tenant.RoleEmployee, authz.ActionWebhookList, false},
		{tenant.RoleEmployee, authz.ActionWebhookRead, false},
		{tenant.RoleEmployee, authz.ActionWebhookUpdate, false},
		{tenant.RoleEmployee, authz.ActionWebhookDelete, false},
		{tenant.RoleEmployee, authz.ActionSubscriptionChange, false},
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

// allActions is every Action this package currently defines. Hand-
// maintained the same way TestRequire's per-role cases above already
// are (Go has no reflection-free way to enumerate a package's own
// constants) — a future phase adding a new Action must add it here too,
// or TestRequire_APIKeyPrincipal_OnlyLeadCreateAllowed/
// TestRequire_PublicFormPrincipal_OnlyLeadCreateAllowed silently stop
// covering it. That's a real gap, not a hypothetical one — twice over
// now: #46 added three ActionAPIKey* constants without adding them
// here (found and backfilled while writing this test), and #85 added
// five ActionForm* constants with the exact same gap, found while
// implementing #87 (this list, and TestRequire's per-role cases above,
// had zero Form entries until now — #85 itself never touched this
// file). #100 (webhook) added its five ActionWebhook* here and to
// TestRequire's per-role cases in the same PR that defined them — the
// gap does not repeat a third time.
var allActions = []authz.Action{
	authz.ActionMembershipList, authz.ActionMembershipUpdateRole, authz.ActionMembershipDeactivate,
	authz.ActionInvitationCreate, authz.ActionInvitationList, authz.ActionInvitationRevoke,
	authz.ActionLeadCreate, authz.ActionLeadRead, authz.ActionLeadUpdate, authz.ActionLeadDelete,
	authz.ActionLeadAssign, authz.ActionLeadConvert,
	authz.ActionActivityCreate, authz.ActionActivityList,
	authz.ActionTaskCreate, authz.ActionTaskRead, authz.ActionTaskUpdate, authz.ActionTaskComplete, authz.ActionTaskDelete,
	authz.ActionCustomerRead, authz.ActionCustomerUpdate, authz.ActionCustomerDelete,
	authz.ActionMetricsRead,
	authz.ActionAPIKeyCreate, authz.ActionAPIKeyList, authz.ActionAPIKeyRevoke,
	authz.ActionDeviceTokenRegister, authz.ActionDeviceTokenDelete,
	authz.ActionFormCreate, authz.ActionFormList, authz.ActionFormRead, authz.ActionFormUpdate, authz.ActionFormDelete,
	authz.ActionWebhookCreate, authz.ActionWebhookList, authz.ActionWebhookRead, authz.ActionWebhookUpdate, authz.ActionWebhookDelete,
	authz.ActionSubscriptionChange,
}

// TestRequire_APIKeyPrincipal_OnlyLeadCreateAllowed is issue #47's
// acceptance criterion, verbatim: "tabel atas seluruh authz.Action —
// setiap action ≠ lead.create ditolak untuk PrincipalAPIKey". Iterating
// allActions rather than hand-picking a few means a new Action added in
// a later phase is denied by default the moment it's added to that
// list above — the whole point of TD §4's design (deny because ABSENT
// from apiKeyScopeFor, not because someone remembered an exception).
func TestRequire_APIKeyPrincipal_OnlyLeadCreateAllowed(t *testing.T) {
	apiKeyCtx := tenant.Context{PrincipalType: tenant.PrincipalAPIKey, Scopes: []string{"leads:write"}}

	for _, action := range allActions {
		t.Run(string(action), func(t *testing.T) {
			err := authz.Require(apiKeyCtx, action)
			if action == authz.ActionLeadCreate {
				if err != nil {
					t.Errorf("expected lead.create to be allowed for an api_key with leads:write, got: %v", err)
				}
				return
			}
			var derr *httpx.DomainError
			if !errors.As(err, &derr) || derr.Code != "insufficient_scope" {
				t.Errorf("expected insufficient_scope for %s, got: %v", action, err)
			}
		})
	}
}

// TestRequire_APIKeyPrincipal_WrongOrMissingScope proves the scope
// check is real — a key that exists but wasn't granted leads:write
// (or was granted nothing) is denied even the one action apiKeyScopeFor
// names.
func TestRequire_APIKeyPrincipal_WrongOrMissingScope(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
	}{
		{"nil scopes", nil},
		{"empty scopes", []string{}},
		{"wrong scope", []string{"leads:read"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := authz.Require(tenant.Context{PrincipalType: tenant.PrincipalAPIKey, Scopes: c.scopes}, authz.ActionLeadCreate)
			var derr *httpx.DomainError
			if !errors.As(err, &derr) || derr.Code != "insufficient_scope" {
				t.Errorf("expected insufficient_scope, got: %v", err)
			}
		})
	}
}

// TestRequire_APIKeyPrincipal_NeverConsultsRoleMap is the structural
// half of Rule #24: an api_key context with Role left at its zero
// value (as it always is — apikey.Usecase.ResolveAPIKey never sets it)
// must be denied for reasons entirely separate from "this role can't do
// this". Setting Role to a real role that WOULD be allowed under the
// role-based map (Owner can do everything) and confirming Require still
// denies it proves PrincipalAPIKey routes to apiKeyScopeFor and never
// silently falls through to permissions[t.Role][action].
func TestRequire_APIKeyPrincipal_NeverConsultsRoleMap(t *testing.T) {
	ctx := tenant.Context{PrincipalType: tenant.PrincipalAPIKey, Role: tenant.RoleOwner, Scopes: nil}
	err := authz.Require(ctx, authz.ActionMembershipList)
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "insufficient_scope" {
		t.Errorf("expected insufficient_scope even though Role=owner would allow membership.list, got: %v", err)
	}
}

// TestRequire_PublicFormPrincipal_OnlyLeadCreateAllowed is issue #87's
// acceptance criterion, verbatim: "public_key tidak bisa memanggil satu
// pun endpoint lain — dibuktikan tabel atas seluruh authz.Action, bukan
// daftar tulisan tangan". Same shape as
// TestRequire_APIKeyPrincipal_OnlyLeadCreateAllowed above — iterating
// allActions means a new Action added in a later phase is denied by
// default the moment it's added to that list, the whole point of TD
// §4's deny-by-absence design for publicFormAllows.
func TestRequire_PublicFormPrincipal_OnlyLeadCreateAllowed(t *testing.T) {
	formCtx := tenant.Context{PrincipalType: tenant.PrincipalPublicForm}

	for _, action := range allActions {
		t.Run(string(action), func(t *testing.T) {
			err := authz.Require(formCtx, action)
			if action == authz.ActionLeadCreate {
				if err != nil {
					t.Errorf("expected lead.create to be allowed for a public form principal, got: %v", err)
				}
				return
			}
			var derr *httpx.DomainError
			if !errors.As(err, &derr) || derr.Code != "forbidden" {
				t.Errorf("expected forbidden for %s, got: %v", action, err)
			}
		})
	}
}

// TestRequire_PublicFormPrincipal_NeverConsultsRoleMap mirrors
// TestRequire_APIKeyPrincipal_NeverConsultsRoleMap — a public form
// context with Role left at its zero value (ResolvePublicKey never
// sets it) must be denied for reasons entirely separate from "this role
// can't do this", proven by setting Role to Owner (which would allow
// everything under the role-based map) and confirming Require still
// routes to publicFormAllows instead.
func TestRequire_PublicFormPrincipal_NeverConsultsRoleMap(t *testing.T) {
	ctx := tenant.Context{PrincipalType: tenant.PrincipalPublicForm, Role: tenant.RoleOwner}
	err := authz.Require(ctx, authz.ActionMembershipList)
	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Errorf("expected forbidden even though Role=owner would allow membership.list, got: %v", err)
	}
}
