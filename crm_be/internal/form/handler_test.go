package form_test

// Integration tests (real Postgres via dbtest) for issue #85's form HTTP
// layer — create/list/get/update/delete as principal user. Nothing here
// exercises the public POST /v1/forms/{public_key}/submit path; that's
// #87's.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
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

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := form.NewUsecase(newTestStore(pool))

	r := gin.New()
	form.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
	return r, pool
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
