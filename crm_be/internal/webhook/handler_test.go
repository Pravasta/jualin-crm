package webhook_test

// Integration tests (real Postgres via dbtest) for the webhook HTTP
// layer (#100) — CRUD of webhook_endpoints as principal user, plus the
// delivery-history and manual-retry routes. The worker that actually
// delivers is #102.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/safedial"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

const testJWTSecret = "webhook-handler-test-jwt-secret-32bytes" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func (s *testStore) InTx(ctx context.Context, fn func(webhook.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(webhook.Repos{
			Endpoint: webhook.New(tx),
			Delivery: webhook.NewDeliveryRepository(tx),
			Audit:    auditlog.New(tx),
		})
	})
}

func (s *testStore) Repos() webhook.Repos {
	return webhook.Repos{
		Endpoint: webhook.New(s.pool),
		Delivery: webhook.NewDeliveryRepository(s.pool),
		Audit:    auditlog.New(s.pool),
	}
}

// newTestRouter wires the real usecase against real Postgres. allowPrivate
// controls the SSRF validator: true lets any well-formed URL through
// without DNS (the default for CRUD tests), false exercises the reject
// path with an IP literal (no DNS needed either).
func newTestRouter(t *testing.T, allowPrivate bool) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := webhook.NewUsecase(&testStore{pool: pool}, safedial.NewValidator(allowPrivate), slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := gin.New()
	webhook.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
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
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func seedOrgOwner(t *testing.T, ctx context.Context, pool *pgxpool.Pool, role tenant.Role) (org, user, membership uuid.UUID) {
	t.Helper()
	org = uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Webhook HTTP Org')`, org); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user = uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Actor')`, user, user.String()+"@example.com"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	membership = uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, $4)`, membership, org, user, role); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return org, user, membership
}

func createEndpoint(t *testing.T, r *gin.Engine, token string) map[string]any {
	t.Helper()
	w := doJSON(r, http.MethodPost, "/v1/webhook-endpoints", token, map[string]any{
		"url":    "https://receiver.example.com/hook",
		"events": []string{"lead.created"},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create endpoint: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Data
}

func TestHandler_Create_OwnerAndAdminAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin} {
		r, pool := newTestRouter(t, true)
		org, user, mem := seedOrgOwner(t, context.Background(), pool, role)
		token := bearerToken(t, user, org, mem, role)

		w := doJSON(r, http.MethodPost, "/v1/webhook-endpoints", token, map[string]any{
			"url": "https://receiver.example.com/hook", "events": []string{"lead.created"},
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("role %s: expected 201, got %d: %s", role, w.Code, w.Body.String())
		}
	}
}

func TestHandler_AllRoutes_ManagerAndEmployeeForbidden(t *testing.T) {
	r, pool := newTestRouter(t, true)
	ownerOrg, ownerUser, ownerMem := seedOrgOwner(t, context.Background(), pool, tenant.RoleOwner)
	ep := createEndpoint(t, r, bearerToken(t, ownerUser, ownerOrg, ownerMem, tenant.RoleOwner))
	epID := ep["id"].(string)

	for _, role := range []tenant.Role{tenant.RoleManager, tenant.RoleEmployee} {
		user := uuid.Must(uuid.NewV7())
		token := bearerToken(t, user, ownerOrg, uuid.Must(uuid.NewV7()), role)

		reqs := []struct {
			method, path string
			body         any
		}{
			{http.MethodPost, "/v1/webhook-endpoints", map[string]any{"url": "https://x.example.com", "events": []string{"lead.created"}}},
			{http.MethodGet, "/v1/webhook-endpoints", nil},
			{http.MethodGet, "/v1/webhook-endpoints/" + epID, nil},
			{http.MethodPatch, "/v1/webhook-endpoints/" + epID, map[string]any{"description": "x"}},
			{http.MethodDelete, "/v1/webhook-endpoints/" + epID, nil},
			{http.MethodGet, "/v1/webhook-endpoints/" + epID + "/deliveries", nil},
		}
		for _, rq := range reqs {
			w := doJSON(r, rq.method, rq.path, token, rq.body)
			if w.Code != http.StatusForbidden {
				t.Errorf("%s %s %s: expected 403, got %d: %s", role, rq.method, rq.path, w.Code, w.Body.String())
			}
		}
	}
}

// TestHandler_Create_SecretShownOnceNeverAgain is Rule #21 at the HTTP
// level: the create response carries the raw whsec_ secret; every
// subsequent read (GET list, GET by id) never does, and the DATABASE
// only ever holds the hash.
func TestHandler_Create_SecretShownOnceNeverAgain(t *testing.T) {
	r, pool := newTestRouter(t, true)
	ctx := context.Background()
	org, user, mem := seedOrgOwner(t, ctx, pool, tenant.RoleOwner)
	token := bearerToken(t, user, org, mem, tenant.RoleOwner)

	data := createEndpoint(t, r, token)
	secret, ok := data["secret"].(string)
	if !ok || secret == "" {
		t.Fatalf("create response missing raw secret, got: %v", data)
	}
	if len(secret) < 10 || secret[:6] != "whsec_" {
		t.Errorf("raw secret malformed: %q", secret)
	}
	epID := data["id"].(string)

	// The database stores only a hash — never the raw secret.
	var storedHash, storedPrefix string
	if err := pool.QueryRow(ctx, `SELECT secret_hash, secret_prefix FROM webhook_endpoints WHERE id = $1`, epID).Scan(&storedHash, &storedPrefix); err != nil {
		t.Fatalf("read stored secret: %v", err)
	}
	if storedHash == secret {
		t.Error("database stored the raw secret, not a hash")
	}
	if len(storedHash) != 64 {
		t.Errorf("stored hash is not 64 hex chars: %q", storedHash)
	}

	// GET by id and GET list never carry a secret field.
	get := doJSON(r, http.MethodGet, "/v1/webhook-endpoints/"+epID, token, nil)
	assertNoSecretField(t, get.Body.Bytes())

	list := doJSON(r, http.MethodGet, "/v1/webhook-endpoints", token, nil)
	assertNoSecretField(t, list.Body.Bytes())
}

func assertNoSecretField(t *testing.T, raw []byte) {
	t.Helper()
	if bytes.Contains(raw, []byte(`"secret"`)) {
		t.Errorf("response leaked a secret field: %s", raw)
	}
}

func TestHandler_Create_RejectsPrivateURL(t *testing.T) {
	r, pool := newTestRouter(t, false) // real validator, no allowPrivate
	org, user, mem := seedOrgOwner(t, context.Background(), pool, tenant.RoleOwner)
	token := bearerToken(t, user, org, mem, tenant.RoleOwner)

	for _, badURL := range []string{
		"http://127.0.0.1/hook",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5:9000/hook",
		"ftp://receiver.example.com/hook",
	} {
		w := doJSON(r, http.MethodPost, "/v1/webhook-endpoints", token, map[string]any{
			"url": badURL, "events": []string{"lead.created"},
		})
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", badURL, w.Code, w.Body.String())
			continue
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body.Error.Code != "webhook_url_not_allowed" {
			t.Errorf("%s: expected code webhook_url_not_allowed, got %q", badURL, body.Error.Code)
		}
	}
}

func TestHandler_Create_RejectsUnknownEvent(t *testing.T) {
	r, pool := newTestRouter(t, true)
	org, user, mem := seedOrgOwner(t, context.Background(), pool, tenant.RoleOwner)
	token := bearerToken(t, user, org, mem, tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/webhook-endpoints", token, map[string]any{
		"url": "https://receiver.example.com/hook", "events": []string{"lead.exploded"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Update_And_Delete(t *testing.T) {
	r, pool := newTestRouter(t, true)
	org, user, mem := seedOrgOwner(t, context.Background(), pool, tenant.RoleOwner)
	token := bearerToken(t, user, org, mem, tenant.RoleOwner)

	ep := createEndpoint(t, r, token)
	epID := ep["id"].(string)

	patch := doJSON(r, http.MethodPatch, "/v1/webhook-endpoints/"+epID, token, map[string]any{
		"is_active": false,
		"events":    []string{"lead.status_changed"},
	})
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: expected 200, got %d: %s", patch.Code, patch.Body.String())
	}
	var patched struct {
		Data struct {
			IsActive bool     `json:"is_active"`
			Events   []string `json:"events"`
		} `json:"data"`
	}
	_ = json.Unmarshal(patch.Body.Bytes(), &patched)
	if patched.Data.IsActive || len(patched.Data.Events) != 1 || patched.Data.Events[0] != "lead.status_changed" {
		t.Errorf("patch not applied: %+v", patched.Data)
	}

	del := doJSON(r, http.MethodDelete, "/v1/webhook-endpoints/"+epID, token, nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", del.Code)
	}
	get := doJSON(r, http.MethodGet, "/v1/webhook-endpoints/"+epID, token, nil)
	if get.Code != http.StatusNotFound {
		t.Errorf("get after delete: expected 404, got %d", get.Code)
	}
}

func TestHandler_ListDeliveries_And_Retry(t *testing.T) {
	r, pool := newTestRouter(t, true)
	ctx := context.Background()
	org, user, mem := seedOrgOwner(t, ctx, pool, tenant.RoleOwner)
	token := bearerToken(t, user, org, mem, tenant.RoleOwner)

	ep := createEndpoint(t, r, token)
	epID := uuid.MustParse(ep["id"].(string))

	// Seed one failed delivery directly.
	deliveryID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, organization_id, endpoint_id, event_type, payload, status, attempt, response_status, error)
		VALUES ($1, $2, $3, 'lead.created', '{"event":"lead.created"}'::jsonb, 'failed', 5, 500, 'server error')`,
		deliveryID, org, epID); err != nil {
		t.Fatalf("seed delivery: %v", err)
	}

	list := doJSON(r, http.MethodGet, "/v1/webhook-endpoints/"+epID.String()+"/deliveries", token, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list deliveries: expected 200, got %d: %s", list.Code, list.Body.String())
	}
	var listBody struct {
		Data []struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			ResponseStatus int    `json:"response_status"`
			Attempt        int    `json:"attempt"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listBody.Meta.Total != 1 || len(listBody.Data) != 1 {
		t.Fatalf("expected exactly one delivery, got %+v", listBody)
	}
	if listBody.Data[0].Status != "failed" || listBody.Data[0].ResponseStatus != 500 || listBody.Data[0].Attempt != 5 {
		t.Errorf("delivery row wrong: %+v", listBody.Data[0])
	}

	// Retry the failed delivery.
	retry := doJSON(r, http.MethodPost, "/v1/webhook-deliveries/"+deliveryID.String()+"/retry", token, nil)
	if retry.Code != http.StatusOK {
		t.Fatalf("retry: expected 200, got %d: %s", retry.Code, retry.Body.String())
	}
	var retried struct {
		Data struct {
			Status  string `json:"status"`
			Attempt int    `json:"attempt"`
		} `json:"data"`
	}
	_ = json.Unmarshal(retry.Body.Bytes(), &retried)
	if retried.Data.Status != "pending" || retried.Data.Attempt != 0 {
		t.Errorf("delivery not re-queued: %+v", retried.Data)
	}

	// Retrying again (now pending) is a 409.
	retryAgain := doJSON(r, http.MethodPost, "/v1/webhook-deliveries/"+deliveryID.String()+"/retry", token, nil)
	if retryAgain.Code != http.StatusConflict {
		t.Errorf("second retry: expected 409, got %d: %s", retryAgain.Code, retryAgain.Body.String())
	}
}
