package metrics_test

// Integration tests (real Postgres via dbtest) for issue #30's HTTP
// layer — same shape as internal/customer's handler_test.go.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/metrics"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const testJWTSecret = "metrics-handler-test-jwt-secret-32bytes" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := metrics.NewUsecase(metrics.New(pool))

	r := gin.New()
	metrics.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
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

func doGet(r *gin.Engine, path, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_Summary_OwnerAllowed_Returns200(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	seedLeadAt(t, ctx, pool, org, nil, "new", time.Now().UTC())

	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	w := doGet(r, "/v1/metrics/summary", token)
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
		t.Errorf("expected total_new=1, got %d", body.Data.TotalNew)
	}
}

// TestHandler_Summary_EmployeeForbidden_Returns403 is issue #30's literal
// acceptance criterion: "Employee → 403 di kedua endpoint metrik".
func TestHandler_Summary_EmployeeForbidden_Returns403(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)

	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	w := doGet(r, "/v1/metrics/summary", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Employees_EmployeeForbidden_Returns403(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)

	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	w := doGet(r, "/v1/metrics/employees", token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Employees_OwnerAllowed_Returns200(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	m := seedMembership(t, ctx, pool, org, "budi@example.com", "Budi")
	seedLeadAt(t, ctx, pool, org, &m, "new", time.Now().UTC())

	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)
	w := doGet(r, "/v1/metrics/employees", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data []struct {
			FullName  string `json:"full_name"`
			LeadCount int    `json:"lead_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].FullName != "Budi" || body.Data[0].LeadCount != 1 {
		t.Errorf("unexpected employees payload: %+v", body.Data)
	}
}

func TestHandler_Unauthenticated_Returns401(t *testing.T) {
	r, _ := newTestRouter(t)
	w := doGet(r, "/v1/metrics/summary", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
