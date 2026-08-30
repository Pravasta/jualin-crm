package main

// End-to-end tests for issue #87 — POST /v1/forms/{public_key}/submit,
// against the exact production router (newRouter) with a genuine
// form.Usecase AND a genuine lead.Usecase wired together through
// newLeadCreatorAdapter — the same composition production traffic goes
// through. internal/form's own tests (usecase_unit_test.go,
// handler_test.go) already cover Submit's business logic against fakes;
// what belongs here is what only exists once form + lead + the real
// leadCreatorAdapter are wired together — genuine persistence into the
// leads table, and the cross-package plumbing #85/#87's own bridge
// interface can't prove on its own. Mirrors public_lead_api_test.go's
// shape exactly.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/formtoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// seedRealForm mirrors seedRealAPIKey — constructs a genuine credential
// through the real form.Usecase (newFormStore), the same composition
// production's own composition root uses, so public_key is exactly
// what an Owner would get from POST /v1/forms.
func seedRealForm(t *testing.T, pool *pgxpool.Pool, org, actorMembership uuid.UUID, allowedOrigins []string) *form.Form {
	t.Helper()
	u := form.NewUsecase(newFormStore(pool), nil, []byte(isolationFormTokenSecret), nil)
	actor := tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalUser, MembershipID: &actorMembership, Role: tenant.RoleOwner}
	f, err := u.Create(context.Background(), actor, form.CreateInput{Name: "Integration Test Form"})
	if err != nil {
		t.Fatalf("seed form: %v", err)
	}
	if len(allowedOrigins) > 0 {
		updated, err := u.Update(context.Background(), actor, f.ID, form.UpdateInput{AllowedOrigins: &allowedOrigins})
		if err != nil {
			t.Fatalf("seed form: set allowed origins: %v", err)
		}
		return updated
	}
	return f
}

// validFormToken mints a token past formtoken's 2-second minimum age —
// same real-sleep tradeoff internal/form's own tests document.
func validFormToken(formID uuid.UUID) string {
	token := formtoken.Issue([]byte(isolationFormTokenSecret), formID)
	time.Sleep(2100 * time.Millisecond)
	return token
}

func doFormPost(r http.Handler, path, origin string, values url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestPublicFormAPI_RealSubmission_CreatesLeadWithSourceAndFormID is
// #87's own acceptance criterion, verbatim: "Submit dari halaman HTML
// di luar jaringan kita → lead muncul dengan source = form + penanda
// form" — proven with a REAL lead.Usecase.Create call underneath (via
// leadCreatorAdapter), not a fake, and a REAL query against the leads
// table afterward.
func TestPublicFormAPI_RealSubmission_CreatesLeadWithSourceAndFormID(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Public Form Org", "owner@example.com")
	f := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"})

	values := url.Values{
		"name":       {"Budi dari Website"},
		"phone":      {"0812xxxxxxx"},
		"form_token": {validFormToken(f.ID)},
		"utm_source": {"facebook"}, // unknown field — must survive into raw_payload
	}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://customer-site.example", values)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var source string
	var sourceFormID uuid.UUID
	var rawPayload []byte
	err := pool.QueryRow(context.Background(),
		`SELECT source, source_form_id, raw_payload FROM leads WHERE id = $1 AND organization_id = $2`,
		body.Data.ID, org,
	).Scan(&source, &sourceFormID, &rawPayload)
	if err != nil {
		t.Fatalf("query created lead: %v", err)
	}
	if source != "form" {
		t.Errorf("expected source=form, got %q", source)
	}
	if sourceFormID != f.ID {
		t.Errorf("expected source_form_id %s, got %s", f.ID, sourceFormID)
	}
	var parsedPayload map[string]string
	if err := json.Unmarshal(rawPayload, &parsedPayload); err != nil {
		t.Fatalf("unmarshal raw_payload: %v", err)
	}
	if parsedPayload["utm_source"] != "facebook" {
		t.Errorf("expected unknown field preserved in raw_payload, got %+v", parsedPayload)
	}

	// submit_count must have moved too — TD §1's dashboard-display
	// counter, incremented as part of the same Submit call.
	var submitCount int
	if err := pool.QueryRow(context.Background(), `SELECT submit_count FROM forms WHERE id = $1`, f.ID).Scan(&submitCount); err != nil {
		t.Fatalf("query submit_count: %v", err)
	}
	if submitCount != 1 {
		t.Errorf("expected submit_count=1, got %d", submitCount)
	}
}

// TestPublicFormAPI_AssignedToMembershipID_SilentlyIgnored proves the
// form-principal branch in lead.Usecase.Create rejects assignment the
// same way the API-key branch does — but since a plain HTML form has no
// way to send a field this codebase doesn't define an <input> for in
// the first place, this test sends it as an EXTRA url-encoded field
// with the same name lead.CreateLeadInput would use, confirming it's
// simply never read (form's handler never populates
// AssignedToMembershipID from the request at all — Rule #5), not that
// it's read-then-rejected. A 201 here (not a 403) is itself the
// assertion: form's own SubmitInput has no such field to even parse.
func TestPublicFormAPI_AssignedToMembershipID_SilentlyIgnored(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Assignment Ignore Org", "owner@example.com")
	f := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"})

	values := url.Values{
		"name":                      {"Budi"},
		"phone":                     {"0812"},
		"form_token":                {validFormToken(f.ID)},
		"assigned_to_membership_id": {ownerMembership.String()},
	}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://customer-site.example", values)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct{ ID string } `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var assignedTo *uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT assigned_to_membership_id FROM leads WHERE id = $1`, body.Data.ID).Scan(&assignedTo); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if assignedTo != nil {
		t.Errorf("expected assigned_to_membership_id to stay nil, got %v", *assignedTo)
	}
}

func corsFormTestConfig() *config.Config {
	cfg := isolationTestConfig()
	cfg.CORSAllowedOrigins = []string{"http://localhost:3000"}
	return cfg
}

// TestPublicFormAPI_CustomerOriginNeverGetsCORSHeaders proves the
// shared httpx.CORS middleware (mounted globally, gates the DASHBOARD's
// cross-origin fetch/XHR calls) is orthogonal to form submission: a
// customer's embed page origin is never in CORSAllowedOrigins (that
// list is the dashboard's own origin, Phase 3 TD §1.1) and gets no
// CORS headers — but the plain application/x-www-form-urlencoded POST
// still succeeds, because a native <form method="post"> is a browser
// "simple request" that was never gated by CORS in the first place
// (CORS restricts JS reading a cross-origin response, not whether a
// simple-content-type form POST is sent or processed at all). The ONE
// thing standing between an arbitrary origin and this endpoint is
// form's own Origin-allowlist check (Usecase.Submit) — proven
// separately by TestHandler_Submit_OriginNotAllowed_Returns403.
func TestPublicFormAPI_CustomerOriginNeverGetsCORSHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	r := newRouter(testLogger(), pool, corsFormTestConfig())

	org, _, ownerMembership := seedOrgOwner(t, pool, "CORS Form Test Org", "owner@example.com")
	f := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"})

	values := url.Values{"name": {"Budi"}, "phone": {"0812"}, "form_token": {validFormToken(f.ID)}}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://customer-site.example", values)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 despite no CORS headers for this origin, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header for a customer origin, got %q", got)
	}
}
