package main

// TestTenantIsolation_* is multi-tenancy.md's lapis 4 — the generic,
// CI-blocking harness proving cross-tenant access returns 404 (never
// 403 — Rule #6). It runs against the FULLY wired router (newRouter,
// real Postgres via dbtest), the same composition production traffic
// goes through, because that's the only way "generic over the route
// list" means anything: testing each domain's Usecase in isolation (as
// the internal/membership and internal/invitation test suites already
// do) can't catch a routing or wiring mistake that only shows up when
// everything is assembled together.
//
// Table-driven over isolationCase — short today (three entries), built
// so Phase 2 appends `lead` cases to the same slice rather than writing
// a new harness.
//
// Quality bar (freeze bagian 5, lapis 4): this harness must be provably
// able to fail. Verified manually during #11's implementation by
// temporarily removing the "AND organization_id = $2" predicate from
// membership.postgresRepository.FindByID and re-running this file — it
// went red. See docs/phases/01-auth-organization/notes.md's "## #11"
// section for the exact procedure and output; the change itself was
// never committed.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/invitation"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const isolationJWTSecret = "isolation-test-jwt-secret-32-bytes-long"

func isolationTestConfig() *config.Config {
	return &config.Config{
		AppEnv:                   "development",
		MailProvider:             "log",
		AppBaseURL:               "http://localhost:3000",
		JWTSecret:                isolationJWTSecret,
		AccessTokenTTL:           15 * time.Minute,
		RefreshTokenTTLDashboard: 720 * time.Hour,
		RefreshTokenTTLMobile:    2160 * time.Hour,
		// Zero would make ratelimit.NewFixedWindow(0, ...) reject every
		// request (Phase 4 #47) — harmless for this file (nothing here
		// calls POST /v1/leads via API key), but a config this test
		// harness hands to newRouter should never carry a value real
		// config.Load() would itself reject as invalid (config.go's
		// validate() requires > 0).
		PublicAPIRateLimit: 100,
		// Zero value "" (not env.Parse's "none" default — this struct is
		// built directly, bypassing envDefault entirely) would hit
		// newPushSender's unreachable default case and panic (Phase 5
		// #68) — every isolation test in this file goes through
		// newRouter, so this has to be set explicitly here too.
		PushProvider: "none",
	}
}

// newIsolationRouter builds the exact production router (newRouter) —
// every domain wired — against a fresh dbtest Postgres instance.
func newIsolationRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	return newRouter(testLogger(), pool, isolationTestConfig()), pool
}

// mintBearerToken issues a real access token signed with the same
// secret newRouter's AuthMiddleware verifies against — bypassing the
// register/verify-email/login HTTP round trip, which isn't what this
// harness is testing. Bearer (not cookie) so CSRF never enters into it.
func mintBearerToken(t *testing.T, userID, orgID, membershipID uuid.UUID, role tenant.Role) string {
	t.Helper()
	tok, err := accesstoken.Issue([]byte(isolationJWTSecret), 15*time.Minute, userID, orgID, membershipID, role)
	if err != nil {
		t.Fatalf("mint access token: %v", err)
	}
	return tok
}

func doIsolationRequest(r *gin.Engine, method, path, bearer string, body any) *httptest.ResponseRecorder {
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

// seedOrgOwner creates an organization with one owner membership,
// directly via each domain's repository — no HTTP round trip, since
// registration itself is #9's concern, already covered there.
func seedOrgOwner(t *testing.T, pool *pgxpool.Pool, orgName, email string) (orgID, userID, membershipID uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	orgID = uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, $2)`, orgID, orgName); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	userID = uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name, email_verified_at) VALUES ($1, $2, 'x', 'Owner', now())`, userID, email); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	repo := membership.New(pool)
	m, err := repo.Create(ctx, tenant.Context{OrganizationID: orgID}, uuid.Must(uuid.NewV7()), userID, tenant.RoleOwner)
	if err != nil {
		t.Fatalf("seed owner membership: %v", err)
	}
	return orgID, userID, m.ID
}

// isolationCase is one entry in the generic route table — a mutating
// by-id endpoint that must 404 (never 403) when the id belongs to
// another organization.
type isolationCase struct {
	name   string
	method string
	path   func(targetID uuid.UUID) string
	body   any
}

func TestTenantIsolation_CrossOrgMutatingByID_Returns404(t *testing.T) {
	r, pool := newIsolationRouter(t)

	orgA, _, ownerAMembershipID := seedOrgOwner(t, pool, "Org A", "owner-a@example.com")
	tokenA := mintBearerToken(t, uuid.Must(uuid.NewV7()), orgA, ownerAMembershipID, tenant.RoleOwner)

	orgB, targetUserB, targetMembershipB := seedOrgOwner(t, pool, "Org B", "owner-b@example.com")
	_ = targetUserB

	invRepo := invitation.New(pool)
	invB := &invitation.Invitation{
		ID:                    uuid.Must(uuid.NewV7()),
		OrganizationID:        orgB,
		Email:                 "invitee-b@example.com",
		Role:                  tenant.RoleEmployee,
		TokenHash:             "isolation-test-hash-" + uuid.Must(uuid.NewV7()).String(),
		InvitedByMembershipID: targetMembershipB,
		ExpiresAt:             time.Now().Add(24 * time.Hour),
	}
	if err := invRepo.Create(t.Context(), invB); err != nil {
		t.Fatalf("seed invitation in org B: %v", err)
	}

	// Phase 2 resources — activates lapis 4 case #1 (cross-org GET-by-id;
	// no domain before Phase 2 had a real one — membership only ever
	// exposed list, not get-by-id) alongside case #2 (mutate), which the
	// three cases above already cover for Phase 1's resources.
	leadB := seedIsolationLead(t, pool, orgB, "new")
	wonLeadB := seedIsolationLead(t, pool, orgB, "won")
	taskB := seedIsolationTask(t, pool, orgB, leadB)
	customerB := seedIsolationCustomer(t, pool, orgB, wonLeadB)

	// Phase 4 (#46) — api_keys. Owner A holds ActionAPIKeyRevoke, so the
	// expected 404 here is proof of tenant scoping, not authz denial.
	apiKeyB := seedIsolationAPIKey(t, pool, orgB)

	// Phase 6 (#85) — forms. Owner A holds ActionFormRead/Update/Delete,
	// so the expected 404 here is proof of tenant scoping, not authz
	// denial — same reasoning as apiKeyB above.
	formB := seedIsolationForm(t, pool, orgB)

	cases := []isolationCase{
		{
			name:   "PATCH /v1/memberships/{id} on another org's membership",
			method: http.MethodPatch,
			path:   func(id uuid.UUID) string { return "/v1/memberships/" + id.String() },
			body:   map[string]string{"role": "manager"},
		},
		{
			name:   "DELETE /v1/memberships/{id} on another org's membership",
			method: http.MethodDelete,
			path:   func(id uuid.UUID) string { return "/v1/memberships/" + id.String() },
		},
		{
			name:   "DELETE /v1/invitations/{id} on another org's invitation",
			method: http.MethodDelete,
			path:   func(id uuid.UUID) string { return "/v1/invitations/" + id.String() },
		},
		{
			name:   "GET /v1/leads/{id} on another org's lead",
			method: http.MethodGet,
			path:   func(id uuid.UUID) string { return "/v1/leads/" + id.String() },
		},
		{
			name:   "PATCH /v1/leads/{id} on another org's lead",
			method: http.MethodPatch,
			path:   func(id uuid.UUID) string { return "/v1/leads/" + id.String() },
			body:   map[string]any{"version": 1, "name": "Hijacked"},
		},
		{
			name:   "DELETE /v1/leads/{id} on another org's lead",
			method: http.MethodDelete,
			path:   func(id uuid.UUID) string { return "/v1/leads/" + id.String() },
		},
		{
			name:   "GET /v1/leads/{id}/activities on another org's lead",
			method: http.MethodGet,
			path:   func(id uuid.UUID) string { return "/v1/leads/" + id.String() + "/activities" },
		},
		{
			name:   "PATCH /v1/tasks/{id} on another org's task",
			method: http.MethodPatch,
			path:   func(id uuid.UUID) string { return "/v1/tasks/" + id.String() },
			body:   map[string]any{"version": 1, "title": "Hijacked"},
		},
		{
			name:   "DELETE /v1/tasks/{id} on another org's task",
			method: http.MethodDelete,
			path:   func(id uuid.UUID) string { return "/v1/tasks/" + id.String() },
		},
		{
			name:   "GET /v1/customers/{id} on another org's customer",
			method: http.MethodGet,
			path:   func(id uuid.UUID) string { return "/v1/customers/" + id.String() },
		},
		{
			name:   "PATCH /v1/customers/{id} on another org's customer",
			method: http.MethodPatch,
			path:   func(id uuid.UUID) string { return "/v1/customers/" + id.String() },
			body:   map[string]any{"name": "Hijacked"},
		},
		{
			name:   "DELETE /v1/customers/{id} on another org's customer",
			method: http.MethodDelete,
			path:   func(id uuid.UUID) string { return "/v1/customers/" + id.String() },
		},
		{
			name:   "DELETE /v1/api-keys/{id} on another org's api key",
			method: http.MethodDelete,
			path:   func(id uuid.UUID) string { return "/v1/api-keys/" + id.String() },
		},
		{
			name:   "GET /v1/forms/{id} on another org's form",
			method: http.MethodGet,
			path:   func(id uuid.UUID) string { return "/v1/forms/" + id.String() },
		},
		{
			name:   "PATCH /v1/forms/{id} on another org's form",
			method: http.MethodPatch,
			path:   func(id uuid.UUID) string { return "/v1/forms/" + id.String() },
			body:   map[string]any{"name": "Hijacked"},
		},
		{
			name:   "DELETE /v1/forms/{id} on another org's form",
			method: http.MethodDelete,
			path:   func(id uuid.UUID) string { return "/v1/forms/" + id.String() },
		},
	}

	targetIDs := map[string]uuid.UUID{
		"PATCH /v1/memberships/{id} on another org's membership":  targetMembershipB,
		"DELETE /v1/memberships/{id} on another org's membership": targetMembershipB,
		"DELETE /v1/invitations/{id} on another org's invitation": invB.ID,
		"GET /v1/leads/{id} on another org's lead":                leadB,
		"PATCH /v1/leads/{id} on another org's lead":              leadB,
		"DELETE /v1/leads/{id} on another org's lead":             leadB,
		"GET /v1/leads/{id}/activities on another org's lead":     leadB,
		"PATCH /v1/tasks/{id} on another org's task":              taskB,
		"DELETE /v1/tasks/{id} on another org's task":             taskB,
		"GET /v1/customers/{id} on another org's customer":        customerB,
		"PATCH /v1/customers/{id} on another org's customer":      customerB,
		"DELETE /v1/customers/{id} on another org's customer":     customerB,
		"DELETE /v1/api-keys/{id} on another org's api key":       apiKeyB,
		"GET /v1/forms/{id} on another org's form":                formB,
		"PATCH /v1/forms/{id} on another org's form":              formB,
		"DELETE /v1/forms/{id} on another org's form":             formB,
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doIsolationRequest(r, tc.method, tc.path(targetIDs[tc.name]), tokenA, tc.body)
			if w.Code != http.StatusNotFound {
				t.Fatalf("expected 404 for cross-org access, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// seedIsolationLead inserts a minimal lead row directly via SQL — this
// file doesn't import internal/lead's repository beyond what's already
// wired into newRouter, and status is easiest to set directly since
// lead.Repository.Create always defaults to 'new'.
func seedIsolationLead(t *testing.T, pool *pgxpool.Pool, org uuid.UUID, status string) uuid.UUID {
	t.Helper()
	ctx := t.Context()
	id := uuid.Must(uuid.NewV7())
	const q = `
		INSERT INTO leads (id, organization_id, lead_number, name, source, status)
		VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), 'Isolation Test Lead', 'manual', $3)`
	if _, err := pool.Exec(ctx, q, id, org, status); err != nil {
		t.Fatalf("seed isolation lead: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE organizations SET next_lead_number = next_lead_number + 1 WHERE id = $1`, org); err != nil {
		t.Fatalf("bump next_lead_number: %v", err)
	}
	return id
}

func seedIsolationTask(t *testing.T, pool *pgxpool.Pool, org, leadID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := t.Context()
	id := uuid.Must(uuid.NewV7())
	const q = `INSERT INTO tasks (id, organization_id, lead_id, title) VALUES ($1, $2, $3, 'Isolation Test Task')`
	if _, err := pool.Exec(ctx, q, id, org, leadID); err != nil {
		t.Fatalf("seed isolation task: %v", err)
	}
	return id
}

// TestTenantIsolation_MetricsAggregate_ScopedToOrganization is Phase 3
// TD §2.5's addition to this harness — the two metrics endpoints don't
// take a resource :id, so they don't fit the by-ID-404 shape every case
// above uses. A leak here is worse than a single wrong row: it exposes
// another tenant's aggregate business shape (freeze bagian 5, lapis 4).
// Verified manually able to fail the same way this file's header
// documents for #11: temporarily changing
// metrics.postgresRepository.Summary's "organization_id = $1" predicate
// to "(organization_id = $1 OR true)" and re-running this test turned it
// red — org A's total_new read 3 instead of 1, counting org B's two
// leads. See notes.md's "## #30" section for the exact procedure; the
// change itself was never committed.
func TestTenantIsolation_MetricsAggregate_ScopedToOrganization(t *testing.T) {
	r, pool := newIsolationRouter(t)

	orgA, _, ownerAMembershipID := seedOrgOwner(t, pool, "Metrics Org A", "metrics-owner-a@example.com")
	tokenA := mintBearerToken(t, uuid.Must(uuid.NewV7()), orgA, ownerAMembershipID, tenant.RoleOwner)
	seedIsolationLead(t, pool, orgA, "new")

	orgB, _, _ := seedOrgOwner(t, pool, "Metrics Org B", "metrics-owner-b@example.com")
	seedIsolationLead(t, pool, orgB, "new")
	seedIsolationLead(t, pool, orgB, "won")

	w := doIsolationRequest(r, http.MethodGet, "/v1/metrics/summary", tokenA, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			TotalNew int `json:"total_new"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.TotalNew != 1 {
		t.Fatalf("expected org A's summary to count only its own lead (total_new=1), got %d — leaked org B's leads", body.Data.TotalNew)
	}
}

func seedIsolationCustomer(t *testing.T, pool *pgxpool.Pool, org, wonLeadID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := t.Context()
	id := uuid.Must(uuid.NewV7())
	const q = `
		INSERT INTO customers (id, organization_id, name, converted_from_lead_id)
		VALUES ($1, $2, 'Isolation Test Customer', $3)`
	if _, err := pool.Exec(ctx, q, id, org, wonLeadID); err != nil {
		t.Fatalf("seed isolation customer: %v", err)
	}
	return id
}

// seedIsolationAPIKey inserts a minimal api_keys row directly via SQL —
// key_id only needs to be unique (uq_api_keys_key_id is global, not
// per-organization), so it's derived from the row's own id rather than
// going through apikey.Generate.
func seedIsolationAPIKey(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := t.Context()
	id := uuid.Must(uuid.NewV7())
	const q = `
		INSERT INTO api_keys (id, organization_id, key_id, secret_hash, key_prefix, name, scopes)
		VALUES ($1, $2, $3, 'isolation-test-hash', 'jln_live_isol', 'Isolation Test Key', ARRAY['leads:write'])`
	keyID := "isol-" + id.String()[:20]
	if _, err := pool.Exec(ctx, q, id, org, keyID); err != nil {
		t.Fatalf("seed isolation api key: %v", err)
	}
	return id
}

// seedIsolationForm inserts a minimal forms row directly via SQL —
// public_key only needs to be unique (uq_forms_public_key is global,
// not per-organization, same exception as api_keys.key_id above), so
// it's derived from the row's own id rather than going through
// form.generate.
func seedIsolationForm(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := t.Context()
	id := uuid.Must(uuid.NewV7())
	const q = `
		INSERT INTO forms (id, organization_id, public_key, name, fields)
		VALUES ($1, $2, $3, 'Isolation Test Form', '{}'::jsonb)`
	publicKey := "pk_isol_" + id.String()[:20]
	if _, err := pool.Exec(ctx, q, id, org, publicKey); err != nil {
		t.Fatalf("seed isolation form: %v", err)
	}
	return id
}

// TestTenantIsolation_MultiMembership_OnlySeesActiveOrgInToken is
// multi-tenancy.md lapis 4 case #5 (ADR-007) — a user with memberships
// in two organizations must not see the other organization's data
// through the one currently active in their token, even though the
// schema permits both memberships to exist simultaneously.
func TestTenantIsolation_MultiMembership_OnlySeesActiveOrgInToken(t *testing.T) {
	r, pool := newIsolationRouter(t)
	ctx := t.Context()

	orgA, sharedUserID, membershipInA := seedOrgOwner(t, pool, "Shared Org A", "shared-user@example.com")

	orgB := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Shared Org B')`, orgB); err != nil {
		t.Fatalf("seed org B: %v", err)
	}
	repo := membership.New(pool)
	if _, err := repo.Create(ctx, tenant.Context{OrganizationID: orgB}, uuid.Must(uuid.NewV7()), sharedUserID, tenant.RoleEmployee); err != nil {
		t.Fatalf("seed membership in org B for the same user: %v", err)
	}

	tokenScopedToA := mintBearerToken(t, sharedUserID, orgA, membershipInA, tenant.RoleOwner)

	w := doIsolationRequest(r, http.MethodGet, "/v1/memberships", tokenScopedToA, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected exactly 1 member visible (org A's owner only), got %d: %s", len(body.Data), w.Body.String())
	}
}

// TestTenantIsolation_DeviceTokenDelete_CrossOrgReturns404 is Phase 5
// #68's device_tokens case. It doesn't fit the generic by-id table
// above — DELETE /v1/device-tokens addresses its target by a token
// VALUE in the request body, not a path segment — so it gets its own
// test, same reasoning as
// TestTenantIsolation_MetricsAggregate_ScopedToOrganization above.
func TestTenantIsolation_DeviceTokenDelete_CrossOrgReturns404(t *testing.T) {
	r, pool := newIsolationRouter(t)

	orgA, _, ownerAMembershipID := seedOrgOwner(t, pool, "Device Org A", "device-owner-a@example.com")
	tokenA := mintBearerToken(t, uuid.Must(uuid.NewV7()), orgA, ownerAMembershipID, tenant.RoleOwner)

	orgB, _, membershipB := seedOrgOwner(t, pool, "Device Org B", "device-owner-b@example.com")
	seedIsolationDeviceToken(t, pool, orgB, membershipB, "isolation-device-token-b")

	w := doIsolationRequest(r, http.MethodDelete, "/v1/device-tokens", tokenA, map[string]string{"token": "isolation-device-token-b"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a cross-org device token delete, got %d: %s", w.Code, w.Body.String())
	}
}

func seedIsolationDeviceToken(t *testing.T, pool *pgxpool.Pool, org, membershipID uuid.UUID, token string) {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	const q = `INSERT INTO device_tokens (id, organization_id, membership_id, token, platform) VALUES ($1, $2, $3, $4, 'android')`
	if _, err := pool.Exec(t.Context(), q, id, org, membershipID, token); err != nil {
		t.Fatalf("seed isolation device token: %v", err)
	}
}
