package form_test

// Integration tests (real Postgres via dbtest) for the form HTTP
// layer — create/list/get/update/delete as principal user (#85), plus
// the public POST /v1/forms/{public_key}/submit path (#87, below the
// "--- submit ---" marker). GET /embed/{public_key} and embed.js are
// #88's, not tested here.

import (
	"bytes"
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/captcha"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/formtoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const testJWTSecret = "form-handler-test-jwt-secret-32bytes" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) form.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(form.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(form.Repos{Form: form.New(tx), Audit: auditlog.New(tx)})
	})
}

func (s *testStore) Repos() form.Repos {
	return form.Repos{Form: form.New(s.pool), Audit: auditlog.New(s.pool)}
}

// generousSubmitLimit is a rate limit high enough that no CRUD test in
// this file (they never call the submit route at all) or submit test
// that isn't SPECIFICALLY exercising rate limiting could ever hit it
// incidentally.
const generousSubmitLimit = 1000

// alwaysOpenPlanGate lets every pre-existing test in this file keep
// exercising the HTTP layer unaffected by subscription's gate (#113) —
// TestHandler_Create_PlanClosed_Returns403 below is the one test that
// swaps it for a closed gate.
type alwaysOpenPlanGate struct{}

func (alwaysOpenPlanGate) RequireChannel(context.Context, tenant.Context, string) error { return nil }

type alwaysClosedPlanGate struct{}

func (alwaysClosedPlanGate) RequireChannel(context.Context, tenant.Context, string) error {
	return &httpx.DomainError{Status: http.StatusForbidden, Code: "plan_upgrade_required", Message: "Paket Anda tidak mencakup kanal ini."}
}

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	r, pool, _ := newTestRouterWithLeadCreator(t, &fakeLeadCreator{})
	return r, pool
}

func newTestRouterWithPlanGate(t *testing.T, gate form.PlanGate) (*gin.Engine, *pgxpool.Pool, *form.Usecase) {
	t.Helper()
	r, pool, u, _ := newTestRouterFull(t, &fakeLeadCreator{}, "none", "", gate)
	return r, pool, u
}

// newTestRouterWithLeadCreator is newTestRouter's Submit-test-facing
// variant — exposes the *form.Usecase's dependencies a submit test
// needs to swap (which lead creator; rate-limit swaps use their own
// dedicated constructor below, same reasoning) without every CRUD test
// above having to know Submit-specific plumbing exists.
// CAPTCHA_PROVIDER=none — embed/CSP tests that need turnstile use
// newTestRouterWithCaptcha instead.
func newTestRouterWithLeadCreator(t *testing.T, leadCreator form.LeadCreator) (*gin.Engine, *pgxpool.Pool, *form.Usecase) {
	t.Helper()
	r, pool, u, _ := newTestRouterFull(t, leadCreator, "none", "", alwaysOpenPlanGate{})
	return r, pool, u
}

// newTestRouterWithCaptcha is the embed-page tests' variant (#88) —
// exposes captchaProvider/turnstileSiteKey, which only the embed
// handler ever consults (rendering the Turnstile widget/script, never
// verification).
func newTestRouterWithCaptcha(t *testing.T, captchaProvider, turnstileSiteKey string) (*gin.Engine, *pgxpool.Pool, *form.Usecase) {
	t.Helper()
	r, pool, u, _ := newTestRouterFull(t, &fakeLeadCreator{}, captchaProvider, turnstileSiteKey, alwaysOpenPlanGate{})
	return r, pool, u
}

func newTestRouterFull(t *testing.T, leadCreator form.LeadCreator, captchaProvider, turnstileSiteKey string, plan form.PlanGate) (*gin.Engine, *pgxpool.Pool, *form.Usecase, *form.Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := form.NewUsecase(newTestStore(pool), captcha.NoopVerifier{}, testFormTokenSecret, leadCreator, plan)

	r := gin.New()
	ipLimit := ratelimit.NewFixedWindow(generousSubmitLimit, time.Minute)
	keyLimit := ratelimit.NewFixedWindow(generousSubmitLimit, time.Minute)
	h := form.NewHandler(u, ipLimit, keyLimit, captchaProvider, turnstileSiteKey)
	h.RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
	return r, pool, u, h
}

func bearerToken(t *testing.T, userID, orgID, membershipID uuid.UUID, role tenant.Role) string {
	t.Helper()
	tok, err := accesstoken.Issue([]byte(testJWTSecret), 15*time.Minute, userID, orgID, membershipID, role)
	if err != nil {
		t.Fatalf("mint access token: %v", err)
	}
	return tok
}

func doJSON(r *gin.Engine, method, path, bearer string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func seedOrgAndOwner(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (org, user, membership uuid.UUID) {
	t.Helper()
	org = uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Test Org')`, org); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user = uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Owner')`, user, user.String()+"@example.com"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	membership = uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'owner')`, membership, org, user); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return org, user, membership
}

func decodeID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Data.ID
}

// --- create: role matrix ---

func TestHandler_Create_OwnerAndAdminAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org := seedOrganization(t, ctx, pool)
		membershipID := seedMembership(t, ctx, pool, org, "actor-"+string(role)+"@example.com", role)
		token := bearerToken(t, uuid.Must(uuid.NewV7()), org, membershipID, role)

		w := doJSON(r, http.MethodPost, "/v1/forms", token, map[string]string{"name": "Website"})
		if w.Code != http.StatusCreated {
			t.Fatalf("role %s: expected 201, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

func TestHandler_Create_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org, userID, _ := seedOrgAndOwner(t, ctx, pool)
		token := bearerToken(t, userID, org, uuid.Must(uuid.NewV7()), role)

		w := doJSON(r, http.MethodPost, "/v1/forms", token, map[string]string{"name": "Website"})
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s: expected 403, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

// TestHandler_Create_PlanClosed_Returns403PlanUpgradeRequired proves
// subscription's gate (#113) is actually wired into the HTTP layer —
// an Owner, otherwise fully entitled by role, is rejected once the plan
// gate closes the channel.
func TestHandler_Create_PlanClosed_Returns403PlanUpgradeRequired(t *testing.T) {
	r, pool, _ := newTestRouterWithPlanGate(t, alwaysClosedPlanGate{})
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/forms", token, map[string]string{"name": "Website"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "plan_upgrade_required" {
		t.Errorf("expected code plan_upgrade_required, got %q", body.Error.Code)
	}
}

// TestHandler_GetPatchDelete_UnaffectedByPlanClosed proves D4: the plan
// gate closes CREATE, never management of a form that already exists —
// GET/PATCH/DELETE keep working on a plan-closed organization.
func TestHandler_GetPatchDelete_UnaffectedByPlanClosed(t *testing.T) {
	rOpen, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(rOpen, http.MethodPost, "/v1/forms", token, map[string]string{"name": "Website"})
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

	// A second router over the SAME pool (not newTestRouterWithPlanGate,
	// which would build its own empty database) so the closed-gate
	// router sees the form that was actually created above.
	closedUsecase := form.NewUsecase(newTestStore(pool), captcha.NoopVerifier{}, testFormTokenSecret, &fakeLeadCreator{}, alwaysClosedPlanGate{})
	rClosed := gin.New()
	form.NewHandler(closedUsecase, ratelimit.NewFixedWindow(generousSubmitLimit, time.Minute), ratelimit.NewFixedWindow(generousSubmitLimit, time.Minute), "none", "").
		RegisterRoutes(rClosed, authn.Middleware(testClaimsParser{}))

	if w := doJSON(rClosed, http.MethodGet, "/v1/forms/"+createdBody.Data.ID, token, nil); w.Code != http.StatusOK {
		t.Errorf("GET with plan closed: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doJSON(rClosed, http.MethodPatch, "/v1/forms/"+createdBody.Data.ID, token, map[string]string{"name": "Renamed"}); w.Code != http.StatusOK {
		t.Errorf("PATCH with plan closed: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doJSON(rClosed, http.MethodDelete, "/v1/forms/"+createdBody.Data.ID, token, nil); w.Code != http.StatusNoContent {
		t.Errorf("DELETE with plan closed: expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Submit_UnaffectedByPlanClosed proves TD §11: the plan gate
// never touches the public submit path — a visitor on a customer's own
// site must not learn anything about that customer's plan state, even
// when the organization's channel is closed.
func TestHandler_Submit_UnaffectedByPlanClosed(t *testing.T) {
	pool := dbtest.NewPool(t)
	org := seedOrganization(t, context.Background(), pool)
	f := seedRealForm(t, context.Background(), pool, org, "pk_plan_closed", []string{"https://example.com"}, form.DefaultFields())

	u := form.NewUsecase(newTestStore(pool), captcha.NoopVerifier{}, testFormTokenSecret, &fakeLeadCreator{}, alwaysClosedPlanGate{})
	r := gin.New()
	form.NewHandler(u, ratelimit.NewFixedWindow(generousSubmitLimit, time.Minute), ratelimit.NewFixedWindow(generousSubmitLimit, time.Minute), "none", "").
		RegisterRoutes(r, authn.Middleware(testClaimsParser{}))

	values := url.Values{
		"name":       {"Budi Santoso"},
		"phone":      {"0812xxxx"},
		"form_token": {validFormToken(f.ID)},
	}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected submit to succeed regardless of plan state, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Create_ResponseCarriesPublicKeyInFull is the HTTP-level
// counterpart to D3: unlike apikey's create response, which is the ONLY
// place a raw secret ever appears, public_key is not a secret at all —
// it must appear in full on every read, not just at creation.
func TestHandler_Create_ResponseCarriesPublicKeyInFull(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(r, http.MethodPost, "/v1/forms", token, map[string]string{"name": "Website"})
	if created.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", created.Code, created.Body.String())
	}
	var body struct {
		Data struct {
			ID        string `json:"id"`
			PublicKey string `json:"public_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.PublicKey == "" {
		t.Fatal("expected the create response to carry a non-empty public_key")
	}

	// GET /v1/forms/{id} must return the SAME public_key in full — not
	// masked, not omitted.
	get := doJSON(r, http.MethodGet, "/v1/forms/"+body.Data.ID, token, nil)
	if get.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", get.Code, get.Body.String())
	}
	var getBody struct {
		Data struct {
			PublicKey string `json:"public_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &getBody); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if getBody.Data.PublicKey != body.Data.PublicKey {
		t.Fatalf("expected GET to return the same public_key %q, got %q", body.Data.PublicKey, getBody.Data.PublicKey)
	}
}

// --- list: role matrix ---

func TestHandler_List_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org, userID, _ := seedOrgAndOwner(t, ctx, pool)
		token := bearerToken(t, userID, org, uuid.Must(uuid.NewV7()), role)

		w := doJSON(r, http.MethodGet, "/v1/forms", token, nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s: expected 403, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

// --- get: cross-org 404 ---

func TestHandler_Get_CrossOrg_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	orgA, userA, membershipA := seedOrgAndOwner(t, ctx, pool)
	tokenA := bearerToken(t, userA, orgA, membershipA, tenant.RoleOwner)
	created := doJSON(r, http.MethodPost, "/v1/forms", tokenA, map[string]string{"name": "Website"})
	id := decodeID(t, created)

	orgB, userB, membershipB := seedOrgAndOwner(t, ctx, pool)
	tokenB := bearerToken(t, userB, orgB, membershipB, tenant.RoleOwner)

	w := doJSON(r, http.MethodGet, "/v1/forms/"+id, tokenB, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-org get, got %d: %s", w.Code, w.Body.String())
	}
}

// --- update: role matrix, cross-org ---

func TestHandler_Update_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org, userID, ownerMembership := seedOrgAndOwner(t, ctx, pool)
		ownerToken := bearerToken(t, userID, org, ownerMembership, tenant.RoleOwner)
		created := doJSON(r, http.MethodPost, "/v1/forms", ownerToken, map[string]string{"name": "Website"})
		id := decodeID(t, created)

		token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), role)
		w := doJSON(r, http.MethodPatch, "/v1/forms/"+id, token, map[string]string{"name": "Renamed"})
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s: expected 403, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

func TestHandler_Update_CrossOrg_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	orgA, userA, membershipA := seedOrgAndOwner(t, ctx, pool)
	tokenA := bearerToken(t, userA, orgA, membershipA, tenant.RoleOwner)
	created := doJSON(r, http.MethodPost, "/v1/forms", tokenA, map[string]string{"name": "Website"})
	id := decodeID(t, created)

	orgB, userB, membershipB := seedOrgAndOwner(t, ctx, pool)
	tokenB := bearerToken(t, userB, orgB, membershipB, tenant.RoleOwner)

	w := doJSON(r, http.MethodPatch, "/v1/forms/"+id, tokenB, map[string]string{"name": "Renamed"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-org update, got %d: %s", w.Code, w.Body.String())
	}
}

// --- delete: role matrix, idempotency, cross-org ---

func TestHandler_Delete_OwnerAllowed_SecondCallIs404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(r, http.MethodPost, "/v1/forms", token, map[string]string{"name": "Website"})
	id := decodeID(t, created)

	first := doJSON(r, http.MethodDelete, "/v1/forms/"+id, token, nil)
	if first.Code != http.StatusNoContent {
		t.Fatalf("first delete: expected 204, got %d: %s", first.Code, first.Body.String())
	}
	// Unlike api_keys' revoke, a deleted form is excluded from FindByID —
	// the second call is a genuine 404, not a repeated 204 (see the doc
	// comment on form.Usecase.Delete).
	second := doJSON(r, http.MethodDelete, "/v1/forms/"+id, token, nil)
	if second.Code != http.StatusNotFound {
		t.Fatalf("second delete: expected 404, got %d: %s", second.Code, second.Body.String())
	}
}

func TestHandler_Delete_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org, userID, ownerMembership := seedOrgAndOwner(t, ctx, pool)
		ownerToken := bearerToken(t, userID, org, ownerMembership, tenant.RoleOwner)
		created := doJSON(r, http.MethodPost, "/v1/forms", ownerToken, map[string]string{"name": "Website"})
		id := decodeID(t, created)

		token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), role)
		w := doJSON(r, http.MethodDelete, "/v1/forms/"+id, token, nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s: expected 403, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

func TestHandler_Delete_CrossOrg_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	orgA, userA, membershipA := seedOrgAndOwner(t, ctx, pool)
	tokenA := bearerToken(t, userA, orgA, membershipA, tenant.RoleOwner)
	created := doJSON(r, http.MethodPost, "/v1/forms", tokenA, map[string]string{"name": "Website"})
	id := decodeID(t, created)

	orgB, userB, membershipB := seedOrgAndOwner(t, ctx, pool)
	tokenB := bearerToken(t, userB, orgB, membershipB, tenant.RoleOwner)

	w := doJSON(r, http.MethodDelete, "/v1/forms/"+id, tokenB, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-org delete, got %d: %s", w.Code, w.Body.String())
	}
}

// --- audit log ---

func TestHandler_CreateAndDelete_RecordAuditLog(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(r, http.MethodPost, "/v1/forms", token, map[string]string{"name": "Website"})
	id := decodeID(t, created)
	doJSON(r, http.MethodDelete, "/v1/forms/"+id, token, nil)

	var actions []string
	rows, err := pool.Query(ctx, `SELECT action FROM audit_logs WHERE organization_id = $1 ORDER BY created_at`, org)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatalf("scan: %v", err)
		}
		actions = append(actions, a)
	}
	if len(actions) != 2 || actions[0] != "form.created" || actions[1] != "form.deleted" {
		t.Fatalf("expected [form.created form.deleted], got %v", actions)
	}
}

// --- submit (Phase 6 #87) ---

// newTestRouterWithLimits is the submit-rate-limit tests' variant —
// small, deliberately-exhaustible limits, unlike generousSubmitLimit.
func newTestRouterWithLimits(t *testing.T, ipLimit, keyLimit int) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := form.NewUsecase(newTestStore(pool), captcha.NoopVerifier{}, testFormTokenSecret, &fakeLeadCreator{}, alwaysOpenPlanGate{})

	r := gin.New()
	form.NewHandler(u, ratelimit.NewFixedWindow(ipLimit, time.Minute), ratelimit.NewFixedWindow(keyLimit, time.Minute), "none", "").
		RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
	return r, pool
}

// seedRealForm inserts a form directly through the real repository
// (bypassing Usecase.Create, which requires an authenticated principal
// user) — submit tests need precise control over AllowedOrigins/Fields
// that going through the full Create flow would only complicate.
func seedRealForm(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, publicKey string, allowedOrigins []string, fields form.Fields) *form.Form {
	t.Helper()
	repo := form.New(pool)
	f := &form.Form{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: org, PublicKey: publicKey,
		Name: "Test Form", Fields: fields, AllowedOrigins: allowedOrigins,
	}
	if err := repo.Create(ctx, tenant.Context{OrganizationID: org}, f); err != nil {
		t.Fatalf("seed form: %v", err)
	}
	return f
}

// validFormToken mints a token past formtoken's 2-second minimum age —
// same real-sleep tradeoff usecase_unit_test.go's own validTokenFor
// documents (formtoken's public API has no injectable clock, by
// design: TD §6 defines Issue/Verify with no time parameter).
func validFormToken(formID uuid.UUID) string {
	token := formtoken.Issue(testFormTokenSecret, formID)
	time.Sleep(2100 * time.Millisecond)
	return token
}

func doFormPost(r *gin.Engine, path, origin string, values url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestHandler_Submit_Success_Returns201 is #87's own acceptance
// criterion at the HTTP layer: a genuine
// application/x-www-form-urlencoded POST (no JSON, no JavaScript
// required) creates a lead. Proves the whole wire contract end to end —
// body parsing, origin allowlist, time-trap, field validation, lead
// creation — through one real HTTP round trip.
func TestHandler_Submit_Success_Returns201(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_submit_success", []string{"https://example.com"}, form.DefaultFields())

	values := url.Values{
		"name":       {"Budi Santoso"},
		"phone":      {"0812xxxx"},
		"form_token": {validFormToken(f.ID)},
	}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values)
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
	if body.Data.ID == "" {
		t.Error("expected a non-empty lead id in the response")
	}
}

// TestHandler_Submit_UnknownFieldsExcludeProtocolFields proves
// buildRawPayload's exclusion list — form_token must never end up
// stored in raw_payload (it's a credential-adjacent value, not
// submission content), while an arbitrary unknown field the customer's
// HTML happens to send DOES get stored (kriteria: "field tak dikenal
// tersimpan di raw_payload"). Inspects what reached leadCreator directly
// (a fake this test owns) rather than querying the real leads table —
// buildRawPayload is form's own handler logic; whether lead.Usecase.Create
// then persists it correctly is cmd/api/public_form_api_test.go's job,
// with a REAL lead.Usecase wired end to end.
func TestHandler_Submit_UnknownFieldsExcludeProtocolFields(t *testing.T) {
	creator := &fakeLeadCreator{}
	r, pool, _ := newTestRouterWithLeadCreator(t, creator)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_raw_payload", []string{"https://example.com"}, form.DefaultFields())
	token := validFormToken(f.ID)

	values := url.Values{
		"name":       {"Budi Santoso"},
		"phone":      {"0812xxxx"},
		"utm_source": {"facebook"}, // unknown field — must survive into raw_payload
		"form_token": {token},      // protocol field — must NOT survive
	}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if len(creator.calls) != 1 {
		t.Fatalf("expected exactly 1 leadCreator call, got %d", len(creator.calls))
	}

	var parsed map[string]string
	if err := json.Unmarshal(creator.calls[0].rawPayload, &parsed); err != nil {
		t.Fatalf("unmarshal raw_payload: %v", err)
	}
	if parsed["utm_source"] != "facebook" {
		t.Errorf("expected utm_source preserved in raw_payload, got %+v", parsed)
	}
	if _, present := parsed["form_token"]; present {
		t.Errorf("expected form_token excluded from raw_payload, got %+v", parsed)
	}
}

func TestHandler_Submit_UnknownPublicKey_Returns404(t *testing.T) {
	r, _ := newTestRouter(t)
	values := url.Values{"name": {"Budi"}}
	w := doFormPost(r, "/v1/forms/pk_does-not-exist/submit", "https://example.com", values)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Submit_OriginNotAllowed_Returns403(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_origin_test", []string{"https://allowed.example.com"}, form.DefaultFields())

	values := url.Values{"name": {"Budi"}, "phone": {"0812"}}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://attacker.example.com", values)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "origin_not_allowed" {
		t.Errorf("expected origin_not_allowed, got %q", body.Error.Code)
	}
}

// TestHandler_Submit_PayloadTooLarge_Returns413 proves the 32KB cap is
// enforced at the HTTP layer (http.MaxBytesReader), not just documented.
func TestHandler_Submit_PayloadTooLarge_Returns413(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_too_large", []string{"https://example.com"}, form.DefaultFields())

	values := url.Values{
		"name":    {"Budi"},
		"message": {strings.Repeat("x", 40*1024)}, // 40KB > the 32KB cap
	}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Submit_HoneypotFilled_IndistinguishableStatusAndShape is
// #87's own acceptance criterion at the HTTP layer: status code and
// response body SHAPE match a genuine success exactly (timing is
// separately documented as NOT fully solved — see Usecase.Submit's own
// doc comment — this test only proves what it can: status/shape).
func TestHandler_Submit_HoneypotFilled_IndistinguishableStatusAndShape(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_honeypot", []string{"https://example.com"}, form.DefaultFields())

	values := url.Values{
		"name":    {"Bot"},
		"phone":   {"0812"},
		"website": {"http://spam.example.com"}, // honeypot field, filled
	}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 (fake success), got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.ID == "" {
		t.Error("expected a non-empty (fake) id, matching a genuine success response's shape")
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE organization_id = $1`, org).Scan(&count); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if count != 0 {
		t.Errorf("expected NO lead created for a honeypot-filled submission, found %d", count)
	}
}

func TestHandler_Submit_RateLimitPerIP_Returns429WithHeaders(t *testing.T) {
	r, pool := newTestRouterWithLimits(t, 1, generousSubmitLimit)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_ip_limit", []string{"https://example.com"}, form.DefaultFields())
	values := url.Values{"name": {"Budi"}, "phone": {"0812"}}

	first := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values)
	if first.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header on the first (successful) response")
	}

	second := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on the 2nd request within a 1-request IP limit, got %d: %s", second.Code, second.Body.String())
	}
	if second.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on a 429 response")
	}
}

func TestHandler_Submit_RateLimitPerForm_Returns429(t *testing.T) {
	r, pool := newTestRouterWithLimits(t, generousSubmitLimit, 1)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_form_limit", []string{"https://example.com"}, form.DefaultFields())
	values := url.Values{"name": {"Budi"}, "phone": {"0812"}, "form_token": {validFormToken(f.ID)}}

	if w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values); w.Code != http.StatusCreated {
		t.Fatalf("expected first submission to succeed, got %d: %s", w.Code, w.Body.String())
	}
	second := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", values)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on the 2nd request within a 1-request per-form limit, got %d: %s", second.Code, second.Body.String())
	}
}

// TestHandler_Submit_RateLimitRunsBeforeBodyIsRead is TD §5's ordering
// requirement, proven rather than assumed: a request that is BOTH
// over the IP rate limit AND carries an oversized body must fail as
// 429, never 413 — if body reading happened first, an attacker's flood
// would cost a full body read every time regardless of the limit.
func TestHandler_Submit_RateLimitRunsBeforeBodyIsRead(t *testing.T) {
	r, pool := newTestRouterWithLimits(t, 1, generousSubmitLimit)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_order_test", []string{"https://example.com"}, form.DefaultFields())

	small := url.Values{"name": {"Budi"}}
	if w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", small); w.Code == http.StatusTooManyRequests {
		t.Fatalf("expected the first request to consume the budget without itself being rate-limited, got %d", w.Code)
	}

	oversized := url.Values{"name": {"Budi"}, "message": {strings.Repeat("x", 40*1024)}}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://example.com", oversized)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 (rate limit wins over body size), got %d: %s", w.Code, w.Body.String())
	}
}

// --- embed page (#88) ---

func doGet(r *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_Embed_Success_Returns200WithHTML(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_embed_success", []string{"https://example.com"}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, `action="/v1/forms/`+f.PublicKey+`/submit"`) {
		t.Errorf("expected the form's action to point at this form's own submit URL, got:\n%s", body)
	}
}

func TestHandler_Embed_UnknownPublicKey_Returns404(t *testing.T) {
	r, _, _ := newTestRouterWithCaptcha(t, "none", "")
	w := doGet(r, "/embed/pk_does-not-exist")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Embed_DeletedForm_Returns404_SameAsUnknown(t *testing.T) {
	r, pool, u := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	_ = u

	created := doJSON(r, http.MethodPost, "/v1/forms", token, map[string]string{"name": "Website"})
	id := decodeID(t, created)
	csrfless := doJSON(r, http.MethodDelete, "/v1/forms/"+id, token, nil)
	if csrfless.Code != http.StatusNoContent {
		t.Fatalf("expected delete to succeed, got %d: %s", csrfless.Code, csrfless.Body.String())
	}

	// The form's public_key is still in the DB row (soft delete never
	// scrubs it) — fetch it directly to confirm /embed/{that key} 404s
	// exactly like a key that never existed.
	var publicKey string
	if err := pool.QueryRow(ctx, `SELECT public_key FROM forms WHERE id = $1`, id).Scan(&publicKey); err != nil {
		t.Fatalf("query public_key: %v", err)
	}

	unknown := doGet(r, "/embed/pk_definitely-never-existed")
	deleted := doGet(r, "/embed/"+publicKey)
	if deleted.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a deleted form's embed page, got %d: %s", deleted.Code, deleted.Body.String())
	}
	if deleted.Body.String() != unknown.Body.String() {
		t.Errorf("expected a deleted form's 404 body to be identical to an unknown key's, got %q vs %q", deleted.Body.String(), unknown.Body.String())
	}
}

// TestHandler_Embed_LabelWithScriptTag_Escaped is #88's own acceptance
// criterion, verbatim: label berisi <script> — html/template's
// contextual escaping must neutralize it, proving text/template was
// never accidentally used anywhere in this render path.
func TestHandler_Embed_LabelWithScriptTag_Escaped(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	malicious := `<script>alert(1)</script>`
	fields := form.Fields{
		form.FieldName: {Enabled: true, Required: true, Label: malicious},
	}
	f := seedRealForm(t, ctx, pool, org, "pk_xss_test", []string{"https://example.com"}, fields)

	w := doGet(r, "/embed/"+f.PublicKey)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, malicious) {
		t.Fatalf("expected the label to be HTML-escaped, but the literal <script> tag survived verbatim in:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("expected an HTML-escaped form of the label, got:\n%s", body)
	}
}

func TestHandler_Embed_CSP_FrameAncestorsFromAllowedOrigins(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_csp_test", []string{"https://a.example.com", "https://b.example.com"}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors https://a.example.com https://b.example.com") {
		t.Errorf("expected frame-ancestors to list both allowed origins, got CSP: %q", csp)
	}
}

// TestHandler_Embed_EmptyAllowlist_FrameAncestorsNone is #88's own
// acceptance criterion: a form nobody has configured an origin for yet
// fails CLOSED (frame-ancestors 'none'), not open.
func TestHandler_Embed_EmptyAllowlist_FrameAncestorsNone(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_empty_allowlist", []string{}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("expected frame-ancestors 'none' for an empty allowlist, got CSP: %q", csp)
	}
}

// TestHandler_Embed_XFrameOptionsNeverSent is #88's own acceptance
// criterion, verbatim.
func TestHandler_Embed_XFrameOptionsNeverSent(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_no_xfo", []string{"https://example.com"}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	if got := w.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("expected X-Frame-Options to never be sent, got %q", got)
	}
}

func TestHandler_Embed_CacheControlNoStore(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_no_store", []string{"https://example.com"}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected Cache-Control: no-store, got %q", got)
	}
}

// TestHandler_Embed_NoCaptcha_NoThirdPartyScript is #88's own
// acceptance criterion as corrected during implementation (see
// notes.md's "## #88" and td.md §7's amended JavaScript row): without
// CAPTCHA there is still a small first-party auto-resize script (D8),
// but NEVER Cloudflare's third-party one.
func TestHandler_Embed_NoCaptcha_NoThirdPartyScript(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_no_captcha", []string{"https://example.com"}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	body := w.Body.String()
	if strings.Contains(body, "challenges.cloudflare.com") {
		t.Errorf("expected no Cloudflare script when CAPTCHA_PROVIDER=none, got:\n%s", body)
	}
	if !strings.Contains(body, "jualin:resize") {
		t.Errorf("expected the first-party auto-resize script to still be present, got:\n%s", body)
	}
	if strings.Contains(body, `class="cf-turnstile"`) {
		t.Errorf("expected no Turnstile widget div when CAPTCHA_PROVIDER=none, got:\n%s", body)
	}
}

func TestHandler_Embed_CaptchaEnabled_RendersTurnstileWidgetAndScript(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "turnstile", "test-site-key-abc")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_captcha_on", []string{"https://example.com"}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	body := w.Body.String()
	if !strings.Contains(body, "challenges.cloudflare.com") {
		t.Errorf("expected Cloudflare's script when CAPTCHA_PROVIDER=turnstile, got:\n%s", body)
	}
	if !strings.Contains(body, `data-sitekey="test-site-key-abc"`) {
		t.Errorf("expected the configured site key in the widget div, got:\n%s", body)
	}
}

// TestHandler_Embed_NonceMatchesCSPAndScriptTags proves the nonce CSP
// asserts is trustworthy is the SAME nonce every <script> tag in the
// body actually carries — a mismatch would make the browser refuse to
// run the very scripts the page depends on (auto-resize, and
// Turnstile's own).
func TestHandler_Embed_NonceMatchesCSPAndScriptTags(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "turnstile", "test-site-key-abc")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_nonce_test", []string{"https://example.com"}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	csp := w.Header().Get("Content-Security-Policy")
	nonceIdx := strings.Index(csp, "'nonce-")
	if nonceIdx == -1 {
		t.Fatalf("expected a nonce in the CSP header, got: %q", csp)
	}
	rest := csp[nonceIdx+len("'nonce-"):]
	nonce := rest[:strings.IndexByte(rest, '\'')]
	if nonce == "" {
		t.Fatal("expected a non-empty nonce")
	}

	body := w.Body.String()
	scriptCount := strings.Count(body, `nonce="`+nonce+`"`)
	totalScriptTags := strings.Count(body, "<script ")
	if scriptCount != totalScriptTags || totalScriptTags == 0 {
		t.Errorf("expected every <script> tag (%d found) to carry the CSP's own nonce %q (%d matched), body:\n%s", totalScriptTags, nonce, scriptCount, body)
	}
}

func TestHandler_Embed_AllowedOriginsJSON_MatchesFormAllowedOrigins(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	origins := []string{"https://a.example.com", "https://b.example.com"}
	f := seedRealForm(t, ctx, pool, org, "pk_json_test", origins, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	body := w.Body.String()
	wantJSON, _ := json.Marshal(origins)
	if !strings.Contains(body, "var allowedOrigins = "+string(wantJSON)) {
		t.Errorf("expected the inline script to embed allowedOrigins as %s, got:\n%s", wantJSON, body)
	}
}

func TestHandler_Embed_OnlyEnabledFieldsRendered(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	fields := form.Fields{
		form.FieldName:    {Enabled: true, Required: true, Label: "Nama"},
		form.FieldCompany: {Enabled: false, Required: false, Label: "Perusahaan"},
	}
	f := seedRealForm(t, ctx, pool, org, "pk_enabled_only", []string{"https://example.com"}, fields)

	w := doGet(r, "/embed/"+f.PublicKey)
	body := w.Body.String()
	if !strings.Contains(body, `name="name"`) {
		t.Errorf("expected the enabled 'name' field to render, got:\n%s", body)
	}
	if strings.Contains(body, `name="company"`) {
		t.Errorf("expected the disabled 'company' field to NOT render, got:\n%s", body)
	}
}

// TestHandler_Embed_FieldOrderIsDeterministic proves fields render in
// AllFieldKeys' fixed order every time — Go map iteration over
// Form.Fields is randomized, so a naive `range fields` implementation
// would shuffle field order between requests.
func TestHandler_Embed_FieldOrderIsDeterministic(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_field_order", []string{"https://example.com"}, form.DefaultFields())

	var firstBody string
	for i := 0; i < 5; i++ {
		w := doGet(r, "/embed/"+f.PublicKey)
		nameIdx := strings.Index(w.Body.String(), `name="name"`)
		phoneIdx := strings.Index(w.Body.String(), `name="phone"`)
		if nameIdx == -1 || phoneIdx == -1 || nameIdx > phoneIdx {
			t.Fatalf("run %d: expected 'name' field before 'phone' field (AllFieldKeys order), got:\n%s", i, w.Body.String())
		}
		if i == 0 {
			firstBody = w.Body.String()
		}
	}
	_ = firstBody
}

func TestHandler_EmbedJS_Returns200WithCorrectCacheControl(t *testing.T) {
	r, _, _ := newTestRouterWithCaptcha(t, "none", "")
	w := doGet(r, "/embed.js")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("expected Cache-Control: public, max-age=3600, got %q", got)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("expected Content-Type text/javascript, got %q", ct)
	}
	body := w.Body.String()
	// Smoke-check the four mandatory D8 safety constraints are present
	// in whatever bytes actually got served — guards against the
	// embedded file being edited later without them.
	for _, want := range []string{"jualin:resize", "e.origin !== EMBED_ORIGIN", "h < 100 || h > 4000", "e.source"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected embed.js to contain %q, got:\n%s", want, body)
		}
	}
}

// TestHandler_Embed_FormTokenIsValidTimeTrapToken proves the rendered
// form_token isn't just SOME string — it's a genuine token that
// formtoken.Verify accepts for this exact form's id once past the
// 2-second minimum age.
func TestHandler_Embed_FormTokenIsValidTimeTrapToken(t *testing.T) {
	r, pool, _ := newTestRouterWithCaptcha(t, "none", "")
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	f := seedRealForm(t, ctx, pool, org, "pk_token_test", []string{"https://example.com"}, form.DefaultFields())

	w := doGet(r, "/embed/"+f.PublicKey)
	body := w.Body.String()
	const marker = `name="form_token" value="`
	idx := strings.Index(body, marker)
	if idx == -1 {
		t.Fatalf("expected a form_token hidden input, got:\n%s", body)
	}
	rest := body[idx+len(marker):]
	tokenValue := rest[:strings.IndexByte(rest, '"')]
	if tokenValue == "" {
		t.Fatal("expected a non-empty form_token")
	}

	time.Sleep(2100 * time.Millisecond)
	if err := formtoken.Verify(testFormTokenSecret, tokenValue, f.ID); err != nil {
		t.Errorf("expected the rendered form_token to verify against this form's id, got: %v", err)
	}
}
