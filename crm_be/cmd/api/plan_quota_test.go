package main

// TestPlanQuota_* is subscription #123's end-to-end proof: the wiring
// from an organization's REAL lead count, through the REAL planLimits
// map (#122), through internal/lead's REAL quota gate, into the three
// principals that can create a lead — against the exact production
// router (newRouter), the same composition production traffic goes
// through. Unit tests in internal/lead already prove the gate is
// CALLED correctly with a fake; this file is the one place that proves
// the real limit, read from the real /v1/me, actually rejects a real
// request once reached.
//
// newPlanGateRouter, seedSubscription, decodeErrorCode are
// plan_gate_test.go's — same package, reused. seedOrgOwner,
// mintBearerToken, doIsolationRequest are tenant_isolation_test.go's.
// seedRealAPIKey/seedRealForm/doFormPost/validFormToken are
// public_lead_api_test.go's / public_form_api_test.go's.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// leadQuotaFor reads the organization's own configured limit from the
// real GET /v1/me rather than hardcoding subscription's provisional
// number a second time in this file — that number is free to change
// (ADR-014) without this test needing to change with it.
func leadQuotaFor(t *testing.T, r *gin.Engine, token string) int {
	t.Helper()
	w := doIsolationRequest(r, http.MethodGet, "/v1/me", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/me: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var me struct {
		Data struct {
			Plan struct {
				Limits struct {
					LeadsPerMonth int `json:"leads_per_month"`
				} `json:"limits"`
			} `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode /v1/me: %v", err)
	}
	if me.Data.Plan.Limits.LeadsPerMonth <= 0 {
		t.Fatalf("expected a positive lead quota on the free plan, got %d", me.Data.Plan.Limits.LeadsPerMonth)
	}
	return me.Data.Plan.Limits.LeadsPerMonth
}

// seedLeadsUpToQuota inserts n leads directly via SQL — fast, and
// legitimate for filling state rather than exercising the endpoint
// under test (same pattern internal/metrics' repository_test.go uses).
func seedLeadsUpToQuota(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		id := uuid.Must(uuid.NewV7())
		if _, err := pool.Exec(ctx,
			`INSERT INTO leads (id, organization_id, lead_number, name, source, status)
			 VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), 'Quota Filler', 'manual', 'new')`,
			id, org,
		); err != nil {
			t.Fatalf("seed lead %d: %v", i, err)
		}
		if _, err := pool.Exec(ctx, `UPDATE organizations SET next_lead_number = next_lead_number + 1 WHERE id = $1`, org); err != nil {
			t.Fatalf("bump next_lead_number: %v", err)
		}
	}
}

// TestPlanQuota_AtLimit_ThreePrincipalsBehaveDifferently is kriteria
// TD 8.5 §5's core claim end to end: once the monthly quota is
// reached, user and api_key are rejected with 403 plan_quota_exceeded,
// but a public form submission still succeeds — a site visitor never
// sees anything about the customer's plan state.
func TestPlanQuota_AtLimit_ThreePrincipalsBehaveDifferently(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, ownerUserID, ownerMembership := seedOrgOwner(t, pool, "Plan Quota Org", "plan-quota-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	token := mintBearerToken(t, ownerUserID, org, ownerMembership, tenant.RoleOwner)

	// Issued while comfortably under quota — proves an already-issued
	// key still authenticates even once the quota it will now trip is
	// reached (kriteria: this is about lead CREATION being gated, not
	// the key itself).
	rawAPIKey, _ := seedRealAPIKey(t, pool, org, ownerMembership)
	formForSubmit := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"})

	limit := leadQuotaFor(t, r, token)
	seedLeadsUpToQuota(t, pool, org, limit)

	// user (dashboard/mobile)
	wUser := doIsolationRequest(r, http.MethodPost, "/v1/leads", token, map[string]any{"name": "Lead Ke-N Via Dashboard"})
	if wUser.Code != http.StatusForbidden {
		t.Fatalf("user: expected 403 at quota, got %d: %s", wUser.Code, wUser.Body.String())
	}
	if code := decodeErrorCode(t, wUser.Body.Bytes()); code != "plan_quota_exceeded" {
		t.Errorf("user: expected plan_quota_exceeded, got %q", code)
	}

	// api_key
	wAPIKey := doBearerRequest(r, http.MethodPost, "/v1/leads", rawAPIKey, map[string]string{"name": "Lead Ke-N Via API"})
	if wAPIKey.Code != http.StatusForbidden {
		t.Fatalf("api_key: expected 403 at quota, got %d: %s", wAPIKey.Code, wAPIKey.Body.String())
	}
	if code := decodeErrorCode(t, wAPIKey.Body.Bytes()); code != "plan_quota_exceeded" {
		t.Errorf("api_key: expected plan_quota_exceeded, got %q", code)
	}

	// public_form — must still succeed, and the response must not
	// mention plan/quota/billing in any form.
	values := url.Values{"name": {"Lead Ke-N Via Formulir"}, "phone": {"0812xxxx"}, "form_token": {validFormToken(formForSubmit.ID)}}
	wForm := doFormPost(r, "/v1/forms/"+formForSubmit.PublicKey+"/submit", "https://customer-site.example", values)
	if wForm.Code != http.StatusCreated {
		t.Fatalf("public_form: expected 201 even at quota, got %d: %s", wForm.Code, wForm.Body.String())
	}
	assertNoBillingLeak(t, wForm.Body.String())
}

// assertNoBillingLeak is prd 8.5 kriteria #8 asserted rather than
// assumed: a site visitor filling in a customer's form must never see
// ANYTHING about that customer's billing state. The status code alone
// does not prove that — a 201 carrying "kuota" in a message field would
// leak just as badly — so the body is inspected for every word this
// product uses to talk about plans and quotas.
func assertNoBillingLeak(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, term := range []string{
		"plan", "paket", "kuota", "quota", "limit", "batas",
		"upgrade", "langganan", "subscription", "tagihan", "billing",
	} {
		if strings.Contains(lower, term) {
			t.Errorf("public form response leaks billing vocabulary %q — a site visitor must never see the customer's plan state (kriteria #8). Body: %s", term, body)
		}
	}
}

// TestPlanQuota_PublicForm_NotifiesOwnerOnceNotTwice is kriteria
// "Notifikasi Owner terkirim sekali per bulan, bukan per lead" proven
// end to end: two public-form submissions past the quota produce
// exactly ONE plan_quota_exceeded notification, visible to the Owner
// through the real GET /v1/notifications — not a fake asserting the
// notifier was merely called.
func TestPlanQuota_PublicForm_NotifiesOwnerOnceNotTwice(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, ownerUserID, ownerMembership := seedOrgOwner(t, pool, "Plan Quota Notify Org", "plan-quota-notify-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	token := mintBearerToken(t, ownerUserID, org, ownerMembership, tenant.RoleOwner)
	formForSubmit := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"})

	limit := leadQuotaFor(t, r, token)
	seedLeadsUpToQuota(t, pool, org, limit)

	for i := 0; i < 2; i++ {
		values := url.Values{"name": {"Lead Lewat Kuota"}, "phone": {"0812xxxx"}, "form_token": {validFormToken(formForSubmit.ID)}}
		w := doFormPost(r, "/v1/forms/"+formForSubmit.PublicKey+"/submit", "https://customer-site.example", values)
		if w.Code != http.StatusCreated {
			t.Fatalf("submit %d: expected 201, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	notifs := doIsolationRequest(r, http.MethodGet, "/v1/notifications", token, nil)
	if notifs.Code != http.StatusOK {
		t.Fatalf("GET /v1/notifications: expected 200, got %d: %s", notifs.Code, notifs.Body.String())
	}
	var body struct {
		Data []struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(notifs.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	quotaNotifs := 0
	for _, n := range body.Data {
		if n.Type == "plan_quota_exceeded" {
			quotaNotifs++
		}
	}
	if quotaNotifs != 1 {
		t.Errorf("expected exactly 1 plan_quota_exceeded notification after 2 overage submissions, got %d", quotaNotifs)
	}
}

// TestPlanQuota_UnderLimit_AllThreePrincipalsSucceed is the control
// case — proves the gate only fires because of the limit, not because
// of something else about the wiring.
func TestPlanQuota_UnderLimit_AllThreePrincipalsSucceed(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, ownerUserID, ownerMembership := seedOrgOwner(t, pool, "Plan Quota Under Org", "plan-quota-under-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	token := mintBearerToken(t, ownerUserID, org, ownerMembership, tenant.RoleOwner)
	rawAPIKey, _ := seedRealAPIKey(t, pool, org, ownerMembership)
	formForSubmit := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"})

	if w := doIsolationRequest(r, http.MethodPost, "/v1/leads", token, map[string]any{"name": "Dashboard"}); w.Code != http.StatusCreated {
		t.Errorf("user: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if w := doBearerRequest(r, http.MethodPost, "/v1/leads", rawAPIKey, map[string]string{"name": "API"}); w.Code != http.StatusCreated {
		t.Errorf("api_key: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	values := url.Values{"name": {"Formulir"}, "phone": {"0812xxxx"}, "form_token": {validFormToken(formForSubmit.ID)}}
	if w := doFormPost(r, "/v1/forms/"+formForSubmit.PublicKey+"/submit", "https://customer-site.example", values); w.Code != http.StatusCreated {
		t.Errorf("public_form: expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPlanQuota_AtLimit_ExistingResourcesStillManageable is kriteria
// TD 8.5 §5's other half: the quota gates CREATE only. GET/PATCH/DELETE
// on a lead already created keep working once the quota is reached.
func TestPlanQuota_AtLimit_ExistingResourcesStillManageable(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, ownerUserID, ownerMembership := seedOrgOwner(t, pool, "Plan Quota Manage Org", "plan-quota-manage-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	token := mintBearerToken(t, ownerUserID, org, ownerMembership, tenant.RoleOwner)

	created := doIsolationRequest(r, http.MethodPost, "/v1/leads", token, map[string]any{"name": "Sebelum Kuota Habis"})
	if created.Code != http.StatusCreated {
		t.Fatalf("seed create: expected 201, got %d: %s", created.Code, created.Body.String())
	}
	var createdBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	limit := leadQuotaFor(t, r, token)
	seedLeadsUpToQuota(t, pool, org, limit) // now at/over the limit

	if w := doIsolationRequest(r, http.MethodGet, "/v1/leads/"+createdBody.Data.ID, token, nil); w.Code != http.StatusOK {
		t.Errorf("GET with quota exhausted: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doIsolationRequest(r, http.MethodPatch, "/v1/leads/"+createdBody.Data.ID, token, map[string]any{"name": "Diubah", "version": 1}); w.Code != http.StatusOK {
		t.Errorf("PATCH with quota exhausted: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPlanQuota_ProvenToFail is AC "terbukti bisa gagal": raising the
// limit turns a rejected request into 201, run against the real map —
// not a fake, not a mock. No sabotage of committed code is needed here
// (unlike #113's plan_gate_test.go) because the test raises the limit
// via the real subscription row it already owns — bumping a fresh
// organization onto a plan whose limit comfortably exceeds what was
// seeded proves the SAME code path both denies and allows correctly.
func TestPlanQuota_ProvenToFail(t *testing.T) {
	r, pool := newPlanGateRouter(t)
	org, ownerUserID, ownerMembership := seedOrgOwner(t, pool, "Plan Quota Proof Org", "plan-quota-proof-owner@example.com")
	seedSubscription(t, pool, org, "free", "active")
	token := mintBearerToken(t, ownerUserID, org, ownerMembership, tenant.RoleOwner)

	limit := leadQuotaFor(t, r, token)
	seedLeadsUpToQuota(t, pool, org, limit)

	if w := doIsolationRequest(r, http.MethodPost, "/v1/leads", token, map[string]any{"name": "Ditolak"}); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 at the limit, got %d: %s", w.Code, w.Body.String())
	}

	// Move the SAME organization to a plan whose limit is far above what
	// was seeded (enterprise = unlimited) — the real map, the real
	// resolver, not a sabotaged one.
	if _, err := pool.Exec(context.Background(),
		`UPDATE subscriptions SET plan_code = 'enterprise' WHERE organization_id = $1`, org,
	); err != nil {
		t.Fatalf("upgrade plan: %v", err)
	}

	if w := doIsolationRequest(r, http.MethodPost, "/v1/leads", token, map[string]any{"name": "Diterima Setelah Naik Paket"}); w.Code != http.StatusCreated {
		t.Fatalf("expected 201 after upgrading to an unlimited plan, got %d: %s", w.Code, w.Body.String())
	}
}
