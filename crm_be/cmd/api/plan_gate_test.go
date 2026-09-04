package main

// TestPlanGate_* is subscription #113's end-to-end proof: the wiring
// from an organization's REAL subscriptions row, through the REAL
// planChannels map (#112), through cmd/api's REAL subscription_gate.go
// bridge, into apikey/form/webhook's Create — against the exact
// production router (newRouter), the same composition production
// traffic goes through. Unit tests in each domain package already prove
// the gate is CALLED correctly with a fake; this file is the one place
// that proves the real map resolves the real wire-contract strings
// ("api_key"/"form"/"webhook") to the right channel (TD §7) and that
// D4's boundary (CREATE only) holds end to end.
//
// seedOrgOwner, newIsolationRouter, mintBearerToken, and
// doIsolationRequest are tenant_isolation_test.go's — same package,
// reused rather than duplicated. seedRealAPIKey/seedRealForm
// (public_lead_api_test.go / public_form_api_test.go) construct their
// OWN local Usecase with alwaysOpenPlanGate — deliberately bypassing the
// production gate to simulate "a credential/form issued before the
// plan closed", which is exactly kriteria §11's scenario.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// newPlanGateRouter is newIsolationRouter's variant with
// WebhookAllowPrivateTargets: true — this file's fixed test URL
// ("https://receiver.example.com/hook") has no real DNS record and the
// sandbox this runs in has no network egress, so safedial's default
// (real DNS resolution) would reject it for a reason that has nothing
// to do with the plan gate under test here. Same relaxation
// internal/webhook/handler_test.go's own newTestRouter(t, true) makes
// for exactly the same reason.
func newPlanGateRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	cfg := isolationTestConfig()
	cfg.WebhookAllowPrivateTargets = true
	return newRouter(testLogger(), pool, cfg), pool
}

// seedSubscription gives org an explicit subscriptions row — seedOrgOwner
// deliberately does not create one (registration's CreateFree is #9's
// concern), so every plan-gate test here starts from a known state
// instead of relying on ErrNoActiveSubscription's implicit "no row"
// closed state, which #112's repository_test.go already covers on its
// own.
func seedSubscription(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, planCode, status string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO subscriptions (id, organization_id, plan_code, status) VALUES ($1, $2, $3, $4)`,
		uuid.Must(uuid.NewV7()), org, planCode, status,
	); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
}

func closeOrgPlan(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE subscriptions SET status = 'past_due' WHERE organization_id = $1`, org,
	); err != nil {
		t.Fatalf("close org plan: %v", err)
	}
}

func decodeErrorCode(t *testing.T, body []byte) string {
	t.Helper()
	var v struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return v.Error.Code
}

// TestPlanGate_OpenPlan_AllThreeChannelsCreatable proves the wire
// contract §7 end to end: "api_key"/"form"/"webhook" — the literals
// apikey/form/webhook's Create calls pass to RequireChannel — resolve
// through the REAL planChannels map to the REAL channel each is meant
// to name. A typo in any of the four places that literal is duplicated
// would make exactly one of these three fail here.
func TestPlanGate_OpenPlan_AllThreeChannelsCreatable(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Plan Gate Open Org", "plan-open-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	token := mintBearerToken(t, uuid.Must(uuid.NewV7()), org, ownerMembership, tenant.RoleOwner)

	cases := []struct {
		name string
		path string
		body map[string]any
	}{
		{"api_key", "/v1/api-keys", map[string]any{"name": "Website"}},
		{"form", "/v1/forms", map[string]any{"name": "Website"}},
		{"webhook", "/v1/webhook-endpoints", map[string]any{"url": "https://receiver.example.com/hook", "events": []string{"lead.created"}}},
	}
	for _, c := range cases {
		w := doIsolationRequest(r, http.MethodPost, c.path, token, c.body)
		if w.Code != http.StatusCreated {
			t.Errorf("%s: expected 201 on an open plan, got %d: %s", c.name, w.Code, w.Body.String())
		}
	}
}

// TestPlanGate_ClosedPlan_AllThreePOSTsRejected is kriteria #3: POST
// /v1/api-keys, /v1/forms, and /v1/webhook-endpoints reject with 403
// plan_upgrade_required once the organization's subscription status
// leaves 'active' — proven through curl-equivalent httptest requests
// against the real router, not just a UI that hides the button.
func TestPlanGate_ClosedPlan_AllThreePOSTsRejected(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Plan Gate Closed Org", "plan-closed-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	closeOrgPlan(t, pool, org)
	token := mintBearerToken(t, uuid.Must(uuid.NewV7()), org, ownerMembership, tenant.RoleOwner)

	cases := []struct {
		name string
		path string
		body map[string]any
	}{
		{"api_key", "/v1/api-keys", map[string]any{"name": "Website"}},
		{"form", "/v1/forms", map[string]any{"name": "Website"}},
		{"webhook", "/v1/webhook-endpoints", map[string]any{"url": "https://receiver.example.com/hook", "events": []string{"lead.created"}}},
	}
	for _, c := range cases {
		w := doIsolationRequest(r, http.MethodPost, c.path, token, c.body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: expected 403 on a closed plan, got %d: %s", c.name, w.Code, w.Body.String())
		}
		if code := decodeErrorCode(t, w.Body.Bytes()); code != "plan_upgrade_required" {
			t.Errorf("%s: expected code plan_upgrade_required, got %q", c.name, code)
		}
	}
}

// TestPlanGate_ClosedPlan_ExistingResourcesStillManageable is kriteria
// D4: the gate closes CREATE only. A resource created while the plan was
// open keeps being readable/editable/deletable after the plan closes —
// closing an already-issued channel would be a downgrade path, and none
// exists yet.
func TestPlanGate_ClosedPlan_ExistingResourcesStillManageable(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Plan Gate D4 Org", "plan-d4-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	token := mintBearerToken(t, uuid.Must(uuid.NewV7()), org, ownerMembership, tenant.RoleOwner)

	keyID := createAndExtractID(t, r, token, "/v1/api-keys", map[string]any{"name": "Website"})
	formID := createAndExtractID(t, r, token, "/v1/forms", map[string]any{"name": "Website"})
	webhookID := createAndExtractID(t, r, token, "/v1/webhook-endpoints", map[string]any{"url": "https://receiver.example.com/hook", "events": []string{"lead.created"}})

	closeOrgPlan(t, pool, org)

	type req struct {
		name, method, path string
		body               map[string]any
		wantCode           int
	}
	reqs := []req{
		{"api_key GET", http.MethodGet, "/v1/api-keys", nil, http.StatusOK},
		{"api_key DELETE", http.MethodDelete, "/v1/api-keys/" + keyID, nil, http.StatusNoContent},
		{"form GET", http.MethodGet, "/v1/forms/" + formID, nil, http.StatusOK},
		{"form PATCH", http.MethodPatch, "/v1/forms/" + formID, map[string]any{"name": "Renamed"}, http.StatusOK},
		{"form DELETE", http.MethodDelete, "/v1/forms/" + formID, nil, http.StatusNoContent},
		{"webhook GET", http.MethodGet, "/v1/webhook-endpoints/" + webhookID, nil, http.StatusOK},
		{"webhook PATCH", http.MethodPatch, "/v1/webhook-endpoints/" + webhookID, map[string]any{"description": "x"}, http.StatusOK},
		{"webhook DELETE", http.MethodDelete, "/v1/webhook-endpoints/" + webhookID, nil, http.StatusNoContent},
	}
	for _, rq := range reqs {
		w := doIsolationRequest(r, rq.method, rq.path, token, rq.body)
		if w.Code != rq.wantCode {
			t.Errorf("%s with plan closed: expected %d, got %d: %s", rq.name, rq.wantCode, w.Code, w.Body.String())
		}
	}
}

// TestPlanGate_ClosedPlan_APIKeyLeadCreateStillWorks is kriteria §11:
// an already-issued API key keeps authenticating POST /v1/leads even
// after the organization's plan closes — only ISSUING a new key is
// gated, never using one that already exists.
func TestPlanGate_ClosedPlan_APIKeyLeadCreateStillWorks(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Plan Gate APIKey Org", "plan-apikey-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	raw, _ := seedRealAPIKey(t, pool, org, ownerMembership) // issued while the plan is open

	closeOrgPlan(t, pool, org)

	w := doBearerRequest(r, http.MethodPost, "/v1/leads", raw, map[string]string{"name": "Budi"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 — an already-issued API key must keep working, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPlanGate_ClosedPlan_PublicFormSubmitStillWorks is kriteria §11's
// other half: a site visitor submitting a customer's already-embedded
// form must never see anything about that customer's plan state.
func TestPlanGate_ClosedPlan_PublicFormSubmitStillWorks(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Plan Gate Form Submit Org", "plan-formsubmit-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	f := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"}) // issued while the plan is open

	closeOrgPlan(t, pool, org)

	values := url.Values{"name": {"Budi Santoso"}, "phone": {"0812xxxx"}, "form_token": {validFormToken(f.ID)}}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://customer-site.example", values)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 — an already-embedded form must keep accepting submissions, got %d: %s", w.Code, w.Body.String())
	}
}

func createAndExtractID(t *testing.T, r *gin.Engine, token, path string, body map[string]any) string {
	t.Helper()
	w := doIsolationRequest(r, http.MethodPost, path, token, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("seed create %s: expected 201, got %d: %s", path, w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created %s: %v", path, err)
	}
	return created.Data.ID
}
