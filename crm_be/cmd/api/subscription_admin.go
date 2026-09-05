package main

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
)

// changePlanRequest is shared by both admin surfaces below — the body
// shape is identical, only who is allowed to send it differs.
type changePlanRequest struct {
	PlanCode string `json:"plan_code"`
}

// registerSubscriptionAdminRoutes mounts POST
// /internal/subscriptions/{organization_id}/plan (Phase 8.5 #124) — the
// first surface in this product authenticated by a bearer TOKEN rather
// than as any principal (user/api_key/public_form). It exists so the
// product owner can move a real customer onto a paid plan before
// payment service integration lands (prd D6), from outside the
// application entirely (curl, a support tool) — never from a button any
// customer can reach.
//
// Registered only when adminToken is non-empty: an empty token means
// this deployment has no way to authenticate the route at all, so it is
// left OFF the router — a 404, not a route nobody can ever pass.
func registerSubscriptionAdminRoutes(r gin.IRouter, planUsecase *subscription.Usecase, audit *auditlog.Repository, adminToken string) {
	if adminToken == "" {
		return
	}

	g := r.Group("/internal/subscriptions")
	g.Use(subscriptionAdminAuth(adminToken))
	g.POST("/:organization_id/plan", func(c *gin.Context) {
		orgID, err := uuid.Parse(c.Param("organization_id"))
		if err != nil {
			httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "organization_id", Code: "invalid_value"}))
			return
		}

		var req changePlanRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.PlanCode == "" {
			httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "plan_code", Code: "required"}))
			return
		}

		// PrincipalSystem (tenant package, defined since Phase 1, never
		// used until now): this caller is not a user, not an api_key,
		// not a public_form — it authenticated with a token that proves
		// nothing about WHO, only that they hold the admin secret.
		t := tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalSystem}

		previousPlanCode, err := planUsecase.AdminChangePlan(c.Request.Context(), t, req.PlanCode)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}

		recordPlanChangeAudit(c.Request.Context(), audit, t, nil, orgID, previousPlanCode, req.PlanCode)

		httpx.OK(c, http.StatusOK, gin.H{
			"organization_id": orgID,
			"plan_code":       req.PlanCode,
		})
	})
}

// registerTestCheckoutRoute mounts POST /v1/subscription/test-checkout
// (Phase 8.5 #124) — an Owner-triggered upgrade to Pro with no real
// payment behind it, for exercising the paid-plan gates before payment
// service integration lands. Always upgrades to subscription.PlanPro
// specifically: this button simulates buying Pro, not an arbitrary
// plan_code an Owner could pick (that would make it a second admin
// surface with a session instead of a token — exactly what Rule §8
// forbids).
//
// Registered only when enabled is true: config.Validate refuses to
// boot with this true in production (ADR-010), so the route existing at
// all already means the deployment intends test checkouts to work here.
func registerTestCheckoutRoute(r gin.IRouter, authMW gin.HandlerFunc, planUsecase *subscription.Usecase, audit *auditlog.Repository, enabled bool) {
	if !enabled {
		return
	}

	g := r.Group("/v1/subscription")
	g.Use(authMW)
	g.POST("/test-checkout", func(c *gin.Context) {
		t := authn.TenantFromContext(c)
		if err := authz.Require(t, authz.ActionSubscriptionChange); err != nil {
			httpx.WriteError(c, err)
			return
		}

		previousPlanCode, err := planUsecase.AdminChangePlan(c.Request.Context(), t, subscription.PlanPro)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}

		recordPlanChangeAudit(c.Request.Context(), audit, t, t.MembershipID, t.OrganizationID, previousPlanCode, subscription.PlanPro)

		httpx.OK(c, http.StatusOK, gin.H{"plan_code": subscription.PlanPro})
	})
}

// recordPlanChangeAudit writes the subscription.plan_changed entry
// AFTER the plan change already committed. Deliberately not atomic with
// AdminChangePlan's write — subscription has no Store (TD 8.5 §1: every
// method there is a single-row statement), and adding one solely to
// wrap this rare, secret-token-gated, manually-triggered admin action
// in a transaction would be exactly the abstraction Aturan #27 warns
// against. A failure here is logged by httpx's own request logging middleware
// (the response still succeeds), not silently lost — and is
// detectable: the plan itself did change, checkable via another
// /internal call or GET /v1/me.
func recordPlanChangeAudit(ctx context.Context, audit *auditlog.Repository, t tenant.Context, actorMembershipID *uuid.UUID, orgID uuid.UUID, previousPlanCode, newPlanCode string) {
	_ = audit.RecordChange(ctx, t, actorMembershipID, "subscription.plan_changed", "subscription", orgID,
		gin.H{"plan_code": previousPlanCode}, gin.H{"plan_code": newPlanCode})
}

// subscriptionAdminAuth checks Authorization: Bearer <adminToken> with a
// constant-time comparison (Aturan #20) — never logged (Aturan #26),
// never sent to any client (Aturan #23). Missing or wrong token both
// respond identically (401 unauthenticated) so a wrong guess cannot be
// distinguished from no attempt at all.
func subscriptionAdminAuth(adminToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		const prefix = "Bearer "
		auth := c.GetHeader("Authorization")
		var given string
		if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
			given = auth[len(prefix):]
		}

		if given == "" || subtle.ConstantTimeCompare([]byte(given), []byte(adminToken)) != 1 {
			httpx.RespondError(c, http.StatusUnauthorized, "unauthenticated", "Token tidak valid.")
			c.Abort()
			return
		}
		c.Next()
	}
}
