package main

// End-to-end tests for issue #47 — POST /v1/leads via a REAL API key,
// against the exact production router (newRouter) with a genuine
// apikey.Usecase, the same composition production traffic goes through.
// internal/lead's own tests (handler_test.go) already cover its OWN
// business logic against a fake authn.APIKeyResolver; what belongs here
// is what only exists once apikey + lead + authn are wired together —
// real credential verification, cross-package RBAC, and CORS.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/apikey"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// seedRealAPIKey creates a genuine credential through apikey.Usecase —
// the same construction production's own composition root uses
// (newAPIKeyStore) — so raw is exactly what a real integrator would
// receive from POST /v1/api-keys.
func seedRealAPIKey(t *testing.T, pool *pgxpool.Pool, org, actorMembership uuid.UUID) (raw string, keyID uuid.UUID) {
	t.Helper()
	u := apikey.NewUsecase(newAPIKeyStore(pool))
	actor := tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalUser, MembershipID: &actorMembership, Role: tenant.RoleOwner}
	k, rawCredential, err := u.Create(context.Background(), actor, apikey.CreateInput{Name: "Integration Test Key"})
	if err != nil {
		t.Fatalf("seed api key: %v", err)
	}
	return rawCredential, k.ID
}

func revokeAPIKey(t *testing.T, pool *pgxpool.Pool, org, actorMembership, keyID uuid.UUID) {
	t.Helper()
	u := apikey.NewUsecase(newAPIKeyStore(pool))
	actor := tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalUser, MembershipID: &actorMembership, Role: tenant.RoleOwner}
	if err := u.Revoke(context.Background(), actor, keyID); err != nil {
		t.Fatalf("revoke api key: %v", err)
	}
}

func doBearerRequest(r *gin.Engine, method, path, bearer string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPublicLeadAPI_RealAPIKey_CreatesLeadWithSourceAndKeyID(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Public API Org", "owner@example.com")
	raw, keyID := seedRealAPIKey(t, pool, org, ownerMembership)

	w := doBearerRequest(r, http.MethodPost, "/v1/leads", raw, map[string]string{"name": "Budi dari Website"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			Source         string `json:"source"`
			SourceAPIKeyID string `json:"source_api_key_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Source != "api" {
		t.Errorf("expected source=api, got %q", body.Data.Source)
	}
	if body.Data.SourceAPIKeyID != keyID.String() {
		t.Errorf("expected source_api_key_id %s, got %q", keyID, body.Data.SourceAPIKeyID)
	}
}

func TestPublicLeadAPI_RevokedKey_Returns401Immediately(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Revoke Test Org", "owner@example.com")
	raw, keyID := seedRealAPIKey(t, pool, org, ownerMembership)

	// Prove it works BEFORE revoking — otherwise a 401 later could just
	// mean the credential never worked at all.
	before := doBearerRequest(r, http.MethodPost, "/v1/leads", raw, map[string]string{"name": "Budi"})
	if before.Code != http.StatusCreated {
		t.Fatalf("expected the key to work before revoke, got %d: %s", before.Code, before.Body.String())
	}

	revokeAPIKey(t, pool, org, ownerMembership, keyID)

	after := doBearerRequest(r, http.MethodPost, "/v1/leads", raw, map[string]string{"name": "Ani"})
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 immediately after revoke, got %d: %s", after.Code, after.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(after.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "invalid_api_key" {
		t.Errorf("expected code invalid_api_key, got %q", body.Error.Code)
	}
}

// TestPublicLeadAPI_EveryFailureReason_IdenticalMessage is the
// acceptance criterion verbatim, at the HTTP layer this time (the
// usecase-level proof is internal/apikey/usecase_unit_test.go): unknown
// key_id, wrong secret, and a revoked key must all produce the exact
// same 401 body — nothing here should let a caller distinguish "this
// key never existed" from "this key existed and was revoked".
func TestPublicLeadAPI_EveryFailureReason_IdenticalMessage(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Identical Message Org", "owner@example.com")
	realRaw, realKeyID := seedRealAPIKey(t, pool, org, ownerMembership)
	revokedRaw, revokedKeyID := seedRealAPIKey(t, pool, org, ownerMembership)
	revokeAPIKey(t, pool, org, ownerMembership, revokedKeyID)
	_ = realKeyID

	// Wrong secret: same key_id as the real key, secret replaced.
	wrongSecretRaw := realRaw[:len(realRaw)-1] + "0"
	if wrongSecretRaw == realRaw {
		wrongSecretRaw = realRaw[:len(realRaw)-1] + "1"
	}

	scenarios := map[string]string{
		"unknown key_id": "jln_live_zzzzzzzzzzzz_" + realRaw[len(realRaw)-43:],
		"wrong secret":   wrongSecretRaw,
		"revoked key":    revokedRaw,
	}

	var messages []string
	for name, raw := range scenarios {
		t.Run(name, func(t *testing.T) {
			w := doBearerRequest(r, http.MethodPost, "/v1/leads", raw, map[string]string{"name": "Budi"})
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error.Code != "invalid_api_key" {
				t.Fatalf("expected code invalid_api_key, got %q", body.Error.Code)
			}
			messages = append(messages, body.Error.Message)
		})
	}
	for i := 1; i < len(messages); i++ {
		if messages[i] != messages[0] {
			t.Errorf("expected every failure message identical, got %q and %q", messages[0], messages[i])
		}
	}
}

// TestPublicLeadAPI_CannotReachAnyOtherEndpoint is Rule #24 proven at
// the HTTP layer, across real cross-domain routing: a real, valid,
// unrevoked API key must fail on every endpoint except POST /v1/leads.
//
// The three non-lead endpoints below sit behind plain authn.Middleware,
// which — deliberately, TD phase 4 §3 — has no notion of an API key
// credential at all; MiddlewareWithAPIKey is mounted ONLY on POST
// /v1/leads (cmd/api/main.go). A jln_* value presented there fails JWT
// parsing and is rejected at AUTHENTICATION (401 authentication_required),
// never reaching authz.Require at all. This is a deliberate, DOCUMENTED
// deviation from this issue's own acceptance-criteria text (which reads
// "→ 403" for all four endpoints) — see notes.md's "## #47" section:
// routing-level exclusion is the stronger form of Rule #24 TD §3 itself
// argues for ("an endpoint that never wires this middleware in cannot
// be reached by an API key no matter what authz.Require would otherwise
// have allowed"), and wiring API-key recognition into every route just
// to get a cosmetically different status code would be strictly worse.
// GET /v1/leads is included as a fourth case for the same reason — it's
// on lead's OWN authMW group, not the publicCreateMW group POST sits on.
func TestPublicLeadAPI_CannotReachAnyOtherEndpoint(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Scope Restriction Org", "owner@example.com")
	raw, _ := seedRealAPIKey(t, pool, org, ownerMembership)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"GET /v1/leads", http.MethodGet, "/v1/leads", nil},
		{"GET /v1/memberships", http.MethodGet, "/v1/memberships", nil},
		{"POST /v1/invitations", http.MethodPost, "/v1/invitations", map[string]string{"email": "x@example.com", "role": "employee"}},
		{"POST /v1/api-keys", http.MethodPost, "/v1/api-keys", map[string]string{"name": "Should not be creatable"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := doBearerRequest(r, c.method, c.path, raw, c.body)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 (rejected at authentication, never reaching authz — see test doc comment), got %d: %s", w.Code, w.Body.String())
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error.Code != "authentication_required" {
				t.Errorf("expected code authentication_required, got %q", body.Error.Code)
			}
		})
	}
}

func corsTestConfig() *config.Config {
	cfg := isolationTestConfig()
	cfg.CORSAllowedOrigins = []string{"http://localhost:3000"}
	return cfg
}

// TestPublicLeadAPI_CORS_CustomerOriginNeverAllowed is TD phase 4 §8
// (keputusan D5) — the public API is never reachable from a browser.
// CORSAllowedOrigins is set to a REAL origin (the dashboard's) rather
// than left empty, so this proves toko-pelanggan.example specifically
// is excluded, not just that "nothing is allowed".
func TestPublicLeadAPI_CORS_CustomerOriginNeverAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	r := newRouter(testLogger(), pool, corsTestConfig())

	org, _, ownerMembership := seedOrgOwner(t, pool, "CORS Test Org", "owner@example.com")
	raw, _ := seedRealAPIKey(t, pool, org, ownerMembership)

	body, _ := json.Marshal(map[string]string{"name": "Budi"})
	req := httptest.NewRequest(http.MethodPost, "/v1/leads", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+raw)
	req.Header.Set("Origin", "https://toko-pelanggan.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	for _, h := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers"} {
		if got := w.Header().Get(h); got != "" {
			t.Errorf("expected no %s header for a customer origin, got %q", h, got)
		}
	}
}

// TestPublicLeadAPI_RawSecretAndPayloadNeverLogged is Rule #26,
// verified against ACTUAL log output rather than by reading the code —
// the same discipline #46's handler_test.go already applied to create.
func TestPublicLeadAPI_RawSecretAndPayloadNeverLogged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	r := newRouter(logger, pool, isolationTestConfig())

	org, _, ownerMembership := seedOrgOwner(t, pool, "Logging Test Org", "owner@example.com")
	raw, _ := seedRealAPIKey(t, pool, org, ownerMembership)

	const secretPayloadMarker = "confidential-lead-note-should-never-be-logged"
	w := doBearerRequest(r, http.MethodPost, "/v1/leads", raw, map[string]string{"name": "Budi", "notes": secretPayloadMarker})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if bytes.Contains(buf.Bytes(), []byte(raw)) {
		t.Errorf("raw API key credential appeared in the request log — Rule #26 violation. Log:\n%s", buf.String())
	}
	if bytes.Contains(buf.Bytes(), []byte(secretPayloadMarker)) {
		t.Errorf("lead payload content appeared in the request log — Rule #26 violation. Log:\n%s", buf.String())
	}

	// Also revoke and try again — a REJECTED request is the case TD §11
	// specifically calls out as tempting to over-log.
	w2 := doBearerRequest(r, http.MethodPost, "/v1/leads", "jln_live_unknownkeyid1_"+raw[len(raw)-43:], nil)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for the malformed follow-up request, got %d", w2.Code)
	}
	if bytes.Contains(buf.Bytes(), []byte(raw)) {
		t.Errorf("raw credential appeared in the log after a FAILED request — Rule #26 violation. Log:\n%s", buf.String())
	}
}
