package apikey_test

// Integration tests (real Postgres via dbtest) for issue #46's api_key
// HTTP layer — create/list/revoke as principal user. Nothing here
// exercises authenticating WITH a jln_* credential; that's #47.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/apikey"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const testJWTSecret = "apikey-handler-test-jwt-secret-32bytes" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) apikey.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(apikey.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(apikey.Repos{APIKey: apikey.New(tx), Audit: auditlog.New(tx)})
	})
}

func (s *testStore) Repos() apikey.Repos {
	return apikey.Repos{APIKey: apikey.New(s.pool), Audit: auditlog.New(s.pool)}
}

// alwaysOpenPlanGate lets every one of this file's pre-existing tests
// keep exercising the HTTP layer unaffected by subscription's gate
// (#113) — TestHandler_Create_PlanClosed_Returns403 below is the one
// test that swaps it for a closed gate.
type alwaysOpenPlanGate struct{}

func (alwaysOpenPlanGate) RequireChannel(context.Context, tenant.Context, string) error { return nil }

type alwaysClosedPlanGate struct{}

func (alwaysClosedPlanGate) RequireChannel(context.Context, tenant.Context, string) error {
	return &httpx.DomainError{Status: http.StatusForbidden, Code: "plan_upgrade_required", Message: "Paket Anda tidak mencakup kanal ini."}
}

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	return newTestRouterWithPlanGate(t, alwaysOpenPlanGate{})
}

func newTestRouterWithPlanGate(t *testing.T, gate apikey.PlanGate) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := apikey.NewUsecase(newTestStore(pool), gate)

	r := gin.New()
	apikey.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
	return r, pool
}

// newLoggingTestRouter is like newTestRouter but wires httpx.RequestID +
// httpx.Logging + httpx.Recovery ahead of the routes, with the logger
// writing into buf — used only by
// TestHandler_Create_RawSecretNeverAppearsInLogs, since that's the one
// test that needs to inspect what actually got logged rather than what
// the response body contains.
func newLoggingTestRouter(t *testing.T) (r *gin.Engine, pool *pgxpool.Pool, buf *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool = dbtest.NewPool(t)
	u := apikey.NewUsecase(newTestStore(pool), alwaysOpenPlanGate{})

	buf = &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))

	r = gin.New()
	r.Use(httpx.RequestID(), httpx.Logging(logger), httpx.Recovery(logger))
	apikey.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
	return r, pool, buf
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

// --- create: role matrix ---

// TestHandler_Create_OwnerAndAdminAllowed mints tokens carrying a
// membership_id that actually exists in memberships — created_by
// enforces a genuine composite FK (fk_api_keys_created_by), so a token
// referencing a membership row that was never seeded would fail Create
// for that reason, not because of authz. Contrast with the
// Forbidden/List/Revoke role-matrix tests below, which never reach
// Create/persist at all and so can use an unseeded membership_id safely.
func TestHandler_Create_OwnerAndAdminAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org := seedOrganization(t, ctx, pool)
		membershipID := seedMembership(t, ctx, pool, org, "actor-"+string(role)+"@example.com", role)
		token := bearerToken(t, uuid.Must(uuid.NewV7()), org, membershipID, role)

		w := doJSON(r, http.MethodPost, "/v1/api-keys", token, map[string]string{"name": "Website"})
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

		w := doJSON(r, http.MethodPost, "/v1/api-keys", token, map[string]string{"name": "Website"})
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s: expected 403, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

// TestHandler_Create_PlanClosed_Returns403PlanUpgradeRequired proves
// subscription's gate (#113) is actually wired into the HTTP layer, not
// just the usecase's unit tests — an Owner, otherwise fully entitled by
// role, is rejected once the plan gate closes the channel.
func TestHandler_Create_PlanClosed_Returns403PlanUpgradeRequired(t *testing.T) {
	r, pool := newTestRouterWithPlanGate(t, alwaysClosedPlanGate{})
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/api-keys", token, map[string]string{"name": "Website"})
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

// TestHandler_ListAndRevoke_UnaffectedByPlanClosed proves D4: the plan
// gate closes CREATE, never management of a credential that already
// exists — GET and DELETE keep working on a plan-closed organization.
func TestHandler_ListAndRevoke_UnaffectedByPlanClosed(t *testing.T) {
	// Seed the key through an open-gate router (creating it is not what
	// this test is about), then reopen the SAME database with a
	// closed-gate router to prove list/revoke ignore the gate.
	rOpen, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(rOpen, http.MethodPost, "/v1/api-keys", token, map[string]string{"name": "Website"})
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

	rClosed := gin.New()
	closedUsecase := apikey.NewUsecase(newTestStore(pool), alwaysClosedPlanGate{})
	apikey.NewHandler(closedUsecase).RegisterRoutes(rClosed, authn.Middleware(testClaimsParser{}))

	if w := doJSON(rClosed, http.MethodGet, "/v1/api-keys", token, nil); w.Code != http.StatusOK {
		t.Errorf("GET with plan closed: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doJSON(rClosed, http.MethodDelete, "/v1/api-keys/"+createdBody.Data.ID, token, nil); w.Code != http.StatusNoContent {
		t.Errorf("DELETE with plan closed: expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Create_ResponseCarriesSecretExactlyOnce is acceptance
// criterion #2's HTTP-level proof: the create response — and ONLY the
// create response — contains the raw secret.
func TestHandler_Create_ResponseCarriesSecretExactlyOnce(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/api-keys", token, map[string]string{"name": "Website"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created struct {
		Data struct {
			ID        string `json:"id"`
			KeyPrefix string `json:"key_prefix"`
			Secret    string `json:"secret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Data.Secret == "" {
		t.Fatal("expected the create response to carry a non-empty secret")
	}
	if created.Data.KeyPrefix == "" {
		t.Error("expected key_prefix to be present")
	}

	// The database row must never hold the plaintext — only a hash of it.
	var secretHash string
	if err := pool.QueryRow(ctx, `SELECT secret_hash FROM api_keys WHERE id = $1`, created.Data.ID).Scan(&secretHash); err != nil {
		t.Fatalf("query secret_hash: %v", err)
	}
	if secretHash == "" || secretHash == created.Data.Secret {
		t.Fatalf("expected secret_hash to be a non-empty hash distinct from the raw secret, got %q for raw %q", secretHash, created.Data.Secret)
	}

	// GET /v1/api-keys must NEVER include a "secret" field on any row —
	// not the empty string, not omitted-but-there: the key must be
	// entirely absent from the JSON.
	list := doJSON(r, http.MethodGet, "/v1/api-keys", token, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", list.Code, list.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(list.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode list envelope: %v", err)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw["data"], &rows); err != nil {
		t.Fatalf("decode list data: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 key in the list, got %d", len(rows))
	}
	if _, present := rows[0]["secret"]; present {
		t.Fatalf("expected list response to never contain a \"secret\" field, got: %s", list.Body.String())
	}
	if _, present := rows[0]["secret_hash"]; present {
		t.Fatalf("expected list response to never contain \"secret_hash\", got: %s", list.Body.String())
	}
}

// --- list: role matrix ---

func TestHandler_List_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org, userID, _ := seedOrgAndOwner(t, ctx, pool)
		token := bearerToken(t, userID, org, uuid.Must(uuid.NewV7()), role)

		w := doJSON(r, http.MethodGet, "/v1/api-keys", token, nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s: expected 403, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

// --- revoke: role matrix, idempotency, cross-org ---

func TestHandler_Revoke_OwnerAllowed_TwiceStaysNoContent(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(r, http.MethodPost, "/v1/api-keys", token, map[string]string{"name": "Website"})
	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	first := doJSON(r, http.MethodDelete, "/v1/api-keys/"+body.Data.ID, token, nil)
	if first.Code != http.StatusNoContent {
		t.Fatalf("first revoke: expected 204, got %d: %s", first.Code, first.Body.String())
	}
	second := doJSON(r, http.MethodDelete, "/v1/api-keys/"+body.Data.ID, token, nil)
	if second.Code != http.StatusNoContent {
		t.Fatalf("second revoke: expected 204 (idempotent per TD §9), got %d: %s", second.Code, second.Body.String())
	}
}

func TestHandler_Revoke_ManagerAndEmployeeForbidden(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org, userID, ownerMembership := seedOrgAndOwner(t, ctx, pool)
		ownerToken := bearerToken(t, userID, org, ownerMembership, tenant.RoleOwner)
		created := doJSON(r, http.MethodPost, "/v1/api-keys", ownerToken, map[string]string{"name": "Website"})
		var body struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}

		token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), role)
		w := doJSON(r, http.MethodDelete, "/v1/api-keys/"+body.Data.ID, token, nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s: expected 403, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

func TestHandler_Revoke_CrossOrg_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	orgA, userA, membershipA := seedOrgAndOwner(t, ctx, pool)
	tokenA := bearerToken(t, userA, orgA, membershipA, tenant.RoleOwner)
	created := doJSON(r, http.MethodPost, "/v1/api-keys", tokenA, map[string]string{"name": "Website"})
	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	orgB, userB, membershipB := seedOrgAndOwner(t, ctx, pool)
	tokenB := bearerToken(t, userB, orgB, membershipB, tenant.RoleOwner)

	w := doJSON(r, http.MethodDelete, "/v1/api-keys/"+body.Data.ID, tokenB, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-org revoke, got %d: %s", w.Code, w.Body.String())
	}
}

// --- audit log ---

func TestHandler_CreateAndRevoke_RecordAuditLog(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(r, http.MethodPost, "/v1/api-keys", token, map[string]string{"name": "Website"})
	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	doJSON(r, http.MethodDelete, "/v1/api-keys/"+body.Data.ID, token, nil)

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
	if len(actions) != 2 || actions[0] != "api_key.created" || actions[1] != "api_key.revoked" {
		t.Fatalf("expected [api_key.created api_key.revoked], got %v", actions)
	}
}

// --- Rule #26: raw secret never logged ---

func TestHandler_Create_RawSecretNeverAppearsInLogs(t *testing.T) {
	r, pool, buf := newLoggingTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/api-keys", token, map[string]string{"name": "Website"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Secret string `json:"secret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Secret == "" {
		t.Fatal("test fixture broken: expected a non-empty secret to search for")
	}

	if bytes.Contains(buf.Bytes(), []byte(body.Data.Secret)) {
		t.Fatalf("raw secret appeared in the request log — Rule #26 violation. Log contents:\n%s", buf.String())
	}
}
