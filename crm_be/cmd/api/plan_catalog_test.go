package main

// TestPlanCatalog_* is #125's end-to-end proof for GET /v1/plans —
// against the exact production router (newRouter), same composition
// production traffic goes through.
//
// No tenant-isolation test here unlike this file's siblings
// (plan_gate_test.go, plan_quota_test.go, subscription_admin_test.go):
// the catalog describes what every plan OFFERS, identical for every
// organization asking (subscription.Catalog() takes no tenant
// argument at all) — there is no cross-org data for a boundary bug to
// leak. authz.ActionSubscriptionRead is the only per-request check,
// and it's covered by the role cases below.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type planCatalogEntryJSON struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	PriceLabel string `json:"price_label"`
	Limits     struct {
		LeadsPerMonth int `json:"leads_per_month"`
		Seats         int `json:"seats"`
	} `json:"limits"`
	Channels map[string]bool `json:"channels"`
}

func decodePlanCatalog(t *testing.T, body []byte) []planCatalogEntryJSON {
	t.Helper()
	var v struct {
		Data []planCatalogEntryJSON `json:"data"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode plan catalog: %v", err)
	}
	return v.Data
}

func TestPlanCatalog_Owner_ReturnsThreePlansInOrder(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, userID, membershipID := seedOrgOwner(t, pool, "Plan Catalog Owner Org", "plan-catalog-owner@example.com")
	token := mintBearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doIsolationRequest(r, http.MethodGet, "/v1/plans", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	plans := decodePlanCatalog(t, w.Body.Bytes())
	wantCodes := []string{"free", "pro", "enterprise"}
	if len(plans) != len(wantCodes) {
		t.Fatalf("expected %d plans, got %d: %+v", len(wantCodes), len(plans), plans)
	}
	for i, code := range wantCodes {
		if plans[i].Code != code {
			t.Errorf("position %d: got code %q, want %q", i, plans[i].Code, code)
		}
		if plans[i].Name == "" {
			t.Errorf("plan %q: expected a non-empty display name", code)
		}
		if plans[i].PriceLabel == "" {
			t.Errorf("plan %q: expected a non-empty price label", code)
		}
	}
}

func TestPlanCatalog_Admin_Succeeds(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, _ := seedOrgOwner(t, pool, "Plan Catalog Admin Org", "plan-catalog-admin-owner@example.com")

	admin := seedExtraMember(t, pool, org, "plan-catalog-admin@example.com", tenant.RoleAdmin)
	token := mintBearerToken(t, admin.userID, org, admin.membershipID, tenant.RoleAdmin)

	w := doIsolationRequest(r, http.MethodGet, "/v1/plans", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPlanCatalog_ManagerAndEmployee_Forbidden is the AC verbatim:
// "Manager/Employee di /subscription → ... tidak tersedia untuk role
// Anda". The dashboard-side "nol panggilan API" half of that AC is
// canViewSubscription's job (lib/subscription-permissions.ts) — this
// proves the backend itself denies if that gate is ever bypassed.
func TestPlanCatalog_ManagerAndEmployee_Forbidden(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, _ := seedOrgOwner(t, pool, "Plan Catalog Denied Org", "plan-catalog-denied-owner@example.com")

	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		member := seedExtraMember(t, pool, org, "plan-catalog-"+string(role)+"@example.com", role)
		token := mintBearerToken(t, member.userID, org, member.membershipID, role)

		w := doIsolationRequest(r, http.MethodGet, "/v1/plans", token, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("role %q: expected 403, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

type seededMember struct {
	userID       uuid.UUID
	membershipID uuid.UUID
}

// seedExtraMember adds a second membership to an org seedOrgOwner
// already created — that helper only ever seeds an Owner, so any test
// needing a different role inserts one directly, the same approach
// subscription_admin_test.go's TestTestCheckout_Enabled_NonOwner_Forbidden
// takes.
func seedExtraMember(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, email string, role tenant.Role) seededMember {
	t.Helper()
	ctx := context.Background()
	userID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, email_verified_at) VALUES ($1, $2, 'x', 'Member', now())`,
		userID, email,
	); err != nil {
		t.Fatalf("seed member user: %v", err)
	}
	m, err := membership.New(pool).Create(ctx, tenant.Context{OrganizationID: org}, uuid.Must(uuid.NewV7()), userID, role)
	if err != nil {
		t.Fatalf("seed member membership: %v", err)
	}
	return seededMember{userID: userID, membershipID: m.ID}
}

// --- contact_url (Phase 8.5 follow-up) ---

// TestPlanCatalog_ContactURL_OmittedWhenUnset locks the "no destination
// yet" shape: the field is ABSENT, not an empty string, so the dashboard
// renders the Enterprise card as plain text rather than an href="".
func TestPlanCatalog_ContactURL_OmittedWhenUnset(t *testing.T) {
	r, pool := newIsolationRouter(t) // isolationTestConfig leaves it empty
	org, userID, membershipID := seedOrgOwner(t, pool, "Contact Unset Org", "contact-unset@example.com")
	token := mintBearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doIsolationRequest(r, http.MethodGet, "/v1/plans", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "contact_url") {
		t.Errorf("expected contact_url to be omitted entirely when unconfigured, got: %s", w.Body.String())
	}
}

// TestPlanCatalog_ContactURL_OnlyOnEnterprise proves the link is scoped
// to the one plan that negotiates (prd D4) — Free and Pro are self-serve
// and must never sprout a "contact us" link.
func TestPlanCatalog_ContactURL_OnlyOnEnterprise(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	cfg := isolationTestConfig()
	cfg.EnterpriseContactURL = "https://wa.me/6281234567890?text=Halo"
	r := newRouter(testLogger(), pool, cfg)

	org, userID, membershipID := seedOrgOwner(t, pool, "Contact Set Org", "contact-set@example.com")
	token := mintBearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doIsolationRequest(r, http.MethodGet, "/v1/plans", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var v struct {
		Data []struct {
			Code       string `json:"code"`
			ContactURL string `json:"contact_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, p := range v.Data {
		switch p.Code {
		case "enterprise":
			if p.ContactURL != cfg.EnterpriseContactURL {
				t.Errorf("enterprise: got contact_url %q, want %q", p.ContactURL, cfg.EnterpriseContactURL)
			}
		default:
			if p.ContactURL != "" {
				t.Errorf("plan %q must not carry a contact_url — only Enterprise negotiates (prd D4), got %q", p.Code, p.ContactURL)
			}
		}
	}
}
