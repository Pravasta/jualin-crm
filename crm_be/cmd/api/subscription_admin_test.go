package main

// TestSubscriptionAdmin_* and TestTestCheckout_* are Phase 8.5 #124's
// end-to-end proof for the two admin surfaces — against the exact
// production router (newRouter), the same composition production
// traffic goes through.
//
// newIsolationRouter's config helpers, seedOrgOwner, mintBearerToken,
// doIsolationRequest are tenant_isolation_test.go's, seedSubscription is
// plan_gate_test.go's — same package, reused rather than duplicated.

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const subscriptionAdminTestToken = "subscription-admin-test-token-32-bytes-min" // #nosec G101 -- test-only value

// newSubscriptionAdminRouter builds the real production router with the
// admin token and test-checkout flag set — both routes exist ONLY when
// their respective config value enables them (subscription_admin.go's
// own doc comments), so most tests in this file need this rather than
// newPlanGateRouter/newIsolationRouter, which leave both off.
func newSubscriptionAdminRouter(t *testing.T, adminToken string, testCheckoutEnabled bool) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	cfg := isolationTestConfig()
	cfg.SubscriptionAdminToken = adminToken
	cfg.SubscriptionTestCheckout = testCheckoutEnabled
	return newRouter(testLogger(), pool, cfg), pool
}

// adminChangePlanRequest reuses doIsolationRequest's Authorization:
// Bearer <token> header — subscriptionAdminAuth expects the exact same
// header shape a JWT bearer would use, just compared against a static
// secret instead of verified as a signed token.
func adminChangePlanRequest(r *gin.Engine, orgID uuid.UUID, bearer string, planCode string) *httptest.ResponseRecorder {
	return doIsolationRequest(r, http.MethodPost, "/internal/subscriptions/"+orgID.String()+"/plan", bearer, map[string]string{"plan_code": planCode})
}

func TestSubscriptionAdmin_NoToken_Returns401(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, subscriptionAdminTestToken, false)
	org, _, _ := seedOrgOwner(t, pool, "Admin No Token Org", "admin-no-token@example.com")

	w := adminChangePlanRequest(r, org, "", "pro")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubscriptionAdmin_WrongToken_Returns401(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, subscriptionAdminTestToken, false)
	org, _, _ := seedOrgOwner(t, pool, "Admin Wrong Token Org", "admin-wrong-token@example.com")

	w := adminChangePlanRequest(r, org, "not-the-real-token-but-32-bytes!", "pro")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSubscriptionAdmin_RightToken_ChangesPlanAndRecordsAudit is the AC
// verbatim: "token benar → paket berubah + baris audit_log".
func TestSubscriptionAdmin_RightToken_ChangesPlanAndRecordsAudit(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, subscriptionAdminTestToken, false)
	org, ownerUserID, ownerMembership := seedOrgOwner(t, pool, "Admin Right Token Org", "admin-right-token@example.com")
	seedSubscription(t, pool, org, "free", "active")

	w := adminChangePlanRequest(r, org, subscriptionAdminTestToken, "pro")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var planCode string
	if err := pool.QueryRow(context.Background(), `SELECT plan_code FROM subscriptions WHERE organization_id = $1`, org).Scan(&planCode); err != nil {
		t.Fatalf("query plan_code: %v", err)
	}
	if planCode != "pro" {
		t.Errorf("expected plan_code %q, got %q", "pro", planCode)
	}

	var auditCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE organization_id = $1 AND action = 'subscription.plan_changed'`, org,
	).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected exactly 1 subscription.plan_changed audit entry, got %d", auditCount)
	}

	// The organization's own session must keep working normally
	// afterward — the admin surface doesn't disturb anything about the
	// organization's own auth.
	token := mintBearerToken(t, ownerUserID, org, ownerMembership, tenant.RoleOwner)
	if w := doIsolationRequest(r, http.MethodGet, "/v1/me", token, nil); w.Code != http.StatusOK {
		t.Errorf("expected the organization's own session to keep working, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubscriptionAdmin_UnknownPlanCode_Rejected(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, subscriptionAdminTestToken, false)
	org, _, _ := seedOrgOwner(t, pool, "Admin Unknown Plan Org", "admin-unknown-plan@example.com")

	w := adminChangePlanRequest(r, org, subscriptionAdminTestToken, "not-a-real-plan")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubscriptionAdmin_TenantIsolation(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, subscriptionAdminTestToken, false)
	orgA, _, _ := seedOrgOwner(t, pool, "Admin Isolation Org A", "admin-isolation-a@example.com")
	orgB, _, _ := seedOrgOwner(t, pool, "Admin Isolation Org B", "admin-isolation-b@example.com")
	seedSubscription(t, pool, orgA, "free", "active")
	seedSubscription(t, pool, orgB, "free", "active")

	if w := adminChangePlanRequest(r, orgA, subscriptionAdminTestToken, "pro"); w.Code != http.StatusOK {
		t.Fatalf("expected 200 for org A, got %d: %s", w.Code, w.Body.String())
	}

	var orgBPlanCode string
	if err := pool.QueryRow(context.Background(), `SELECT plan_code FROM subscriptions WHERE organization_id = $1`, orgB).Scan(&orgBPlanCode); err != nil {
		t.Fatalf("query org B: %v", err)
	}
	if orgBPlanCode != "free" {
		t.Errorf("expected org B untouched (still free), got %q", orgBPlanCode)
	}
}

// TestSubscriptionAdmin_RouteNotRegisteredWhenTokenEmpty proves the
// route does not exist at all — a 404 from Gin's own NoRoute handler,
// not a 401 from a middleware guarding a route nobody could ever pass.
func TestSubscriptionAdmin_RouteNotRegisteredWhenTokenEmpty(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, "", false)
	org, _, _ := seedOrgOwner(t, pool, "Admin Disabled Org", "admin-disabled@example.com")

	w := adminChangePlanRequest(r, org, subscriptionAdminTestToken, "pro")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (route not registered), got %d: %s", w.Code, w.Body.String())
	}
}

// TestSubscriptionAdmin_TokenNeverLogged is Aturan #26, verified against
// ACTUAL log output rather than trusted from reading the code.
func TestSubscriptionAdmin_TokenNeverLogged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	cfg := isolationTestConfig()
	cfg.SubscriptionAdminToken = subscriptionAdminTestToken
	r := newRouter(logger, pool, cfg)

	org, _, _ := seedOrgOwner(t, pool, "Admin Log Org", "admin-log@example.com")
	seedSubscription(t, pool, org, "free", "active")

	// One failing attempt (wrong token) and one succeeding attempt (right
	// token) — the raw token must appear in neither log line.
	adminChangePlanRequest(r, org, "some-wrong-token-but-32-bytes-ok", "pro")
	adminChangePlanRequest(r, org, subscriptionAdminTestToken, "pro")

	if bytes.Contains(buf.Bytes(), []byte(subscriptionAdminTestToken)) {
		t.Errorf("the admin token appeared in the request log — Rule #26 violation. Log:\n%s", buf.String())
	}
}

// --- test-checkout ---

func TestTestCheckout_Disabled_Returns404(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, "", false)
	org, ownerUserID, ownerMembership := seedOrgOwner(t, pool, "Test Checkout Disabled Org", "checkout-disabled@example.com")
	token := mintBearerToken(t, ownerUserID, org, ownerMembership, tenant.RoleOwner)

	w := doIsolationRequest(r, http.MethodPost, "/v1/subscription/test-checkout", token, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (route not registered), got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestCheckout_Enabled_Owner_UpgradesToProAndRecordsAudit(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, "", true)
	org, ownerUserID, ownerMembership := seedOrgOwner(t, pool, "Test Checkout Owner Org", "checkout-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	token := mintBearerToken(t, ownerUserID, org, ownerMembership, tenant.RoleOwner)

	w := doIsolationRequest(r, http.MethodPost, "/v1/subscription/test-checkout", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var planCode string
	if err := pool.QueryRow(context.Background(), `SELECT plan_code FROM subscriptions WHERE organization_id = $1`, org).Scan(&planCode); err != nil {
		t.Fatalf("query plan_code: %v", err)
	}
	if planCode != "pro" {
		t.Errorf("expected plan_code %q, got %q", "pro", planCode)
	}

	var auditCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE organization_id = $1 AND action = 'subscription.plan_changed'`, org,
	).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected exactly 1 subscription.plan_changed audit entry, got %d", auditCount)
	}
}

// TestTestCheckout_Enabled_NonOwner_Forbidden proves authz.Require gates
// this route (ActionSubscriptionChange, Owner only) — the first action
// in this codebase where Admin does not mirror Owner.
func TestTestCheckout_Enabled_NonOwner_Forbidden(t *testing.T) {
	r, pool := newSubscriptionAdminRouter(t, "", true)
	org, _, _ := seedOrgOwner(t, pool, "Test Checkout Admin Org", "checkout-owner-org@example.com")
	seedSubscription(t, pool, org, "free", "active")

	ctx := context.Background()
	adminUserID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, email_verified_at) VALUES ($1, $2, 'x', 'Admin', now())`,
		adminUserID, "checkout-admin@example.com",
	); err != nil {
		t.Fatalf("seed admin user: %v", err)
	}
	m, err := membership.New(pool).Create(ctx, tenant.Context{OrganizationID: org}, uuid.Must(uuid.NewV7()), adminUserID, tenant.RoleAdmin)
	if err != nil {
		t.Fatalf("seed admin membership: %v", err)
	}

	token := mintBearerToken(t, adminUserID, org, m.ID, tenant.RoleAdmin)

	w := doIsolationRequest(r, http.MethodPost, "/v1/subscription/test-checkout", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden for Admin (Owner-only action), got %d: %s", w.Code, w.Body.String())
	}
}

// TestSubscriptionAdmin_UnknownOrganization_Returns404 is the /internal/
// surface's likeliest mistake, since organization_id there is typed by
// hand into the path rather than taken from a principal. It answered
// 500 internal_error until this was fixed.
func TestSubscriptionAdmin_UnknownOrganization_Returns404(t *testing.T) {
	r, _ := newSubscriptionAdminRouter(t, subscriptionAdminTestToken, false)

	w := adminChangePlanRequest(r, uuid.Must(uuid.NewV7()), subscriptionAdminTestToken, "pro")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an organization that does not exist, got %d: %s", w.Code, w.Body.String())
	}
}
