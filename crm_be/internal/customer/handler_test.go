package customer_test

// Integration tests (real Postgres via dbtest) for issue #23's
// conversion + customer HTTP layer.

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

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/customer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const testJWTSecret = "customer-handler-test-jwt-secret-32byt" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) customer.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(customer.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(customer.Repos{Customer: customer.New(tx), Activity: activity.NewRecorder(tx)})
	})
}

func (s *testStore) Repos() customer.Repos {
	return customer.Repos{Customer: customer.New(s.pool), Activity: activity.NewRecorder(s.pool)}
}

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := customer.NewUsecase(newTestStore(pool))

	r := gin.New()
	customer.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
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

func seedEmployeeMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID) uuid.UUID {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Employee')`, userID, userID.String()+"@example.com"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	membershipID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'employee')`, membershipID, org, userID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return membershipID
}

// seedLead (email param) and seedOrganization are defined in
// repository_test.go (same package) and reused here.

func extractID(t *testing.T, w *httptest.ResponseRecorder) string {
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

func TestHandler_Convert_WonLead_Returns201(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil, "won", "Budi Santoso", nil)

	w := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/convert", token, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			Name                string `json:"name"`
			ConvertedFromLeadID string `json:"converted_from_lead_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Name != "Budi Santoso" || body.Data.ConvertedFromLeadID != leadID.String() {
		t.Errorf("expected customer copied from lead, got %+v", body.Data)
	}

	var activityCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activities WHERE lead_id = $1 AND type = 'lead_converted'`, leadID).Scan(&activityCount); err != nil {
		t.Fatalf("query activities: %v", err)
	}
	if activityCount != 1 {
		t.Errorf("expected 1 lead_converted activity, got %d", activityCount)
	}

	var leadStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM leads WHERE id = $1`, leadID).Scan(&leadStatus); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if leadStatus != "won" {
		t.Errorf("expected lead to remain won, got %q", leadStatus)
	}
}

func TestHandler_Convert_NotWon_Returns422(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil, "new", "Budi", nil)

	w := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/convert", token, nil)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Convert_Twice_Returns409(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil, "won", "Budi", nil)

	first := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/convert", token, nil)
	if first.Code != http.StatusCreated {
		t.Fatalf("first convert failed: %d: %s", first.Code, first.Body.String())
	}

	w := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/convert", token, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Update_DoesNotChangeOriginatingLead is the literal
// acceptance criterion: "mengubah nama customer tidak mengubah lead
// asalnya" — data is copied, not referenced.
func TestHandler_Update_DoesNotChangeOriginatingLead(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil, "won", "Original Name", nil)

	converted := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/convert", token, nil)
	customerID := extractID(t, converted)

	w := doJSON(r, http.MethodPatch, "/v1/customers/"+customerID, token, map[string]string{"name": "Renamed Customer"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var leadName string
	if err := pool.QueryRow(ctx, `SELECT name FROM leads WHERE id = $1`, leadID).Scan(&leadName); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if leadName != "Original Name" {
		t.Errorf("expected the lead's name to be unchanged, got %q", leadName)
	}
}

func TestHandler_Employee_ReadCustomerFromOtherEmployeesLead_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, ownerUser, ownerMembership := seedOrgAndOwner(t, ctx, pool)
	ownerToken := bearerToken(t, ownerUser, org, ownerMembership, tenant.RoleOwner)

	otherEmployee := seedEmployeeMembership(t, ctx, pool, org)
	leadID := seedLead(t, ctx, pool, org, &otherEmployee, "won", "Budi", nil)
	converted := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/convert", ownerToken, nil)
	customerID := extractID(t, converted)

	me := uuid.Must(uuid.NewV7())
	myToken := bearerToken(t, me, org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	w := doJSON(r, http.MethodGet, "/v1/customers/"+customerID, myToken, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Convert_ManagerForbidden(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, _, _ := seedOrgAndOwner(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil, "won", "Budi", nil)

	managerToken := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleManager)
	w := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/convert", managerToken, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
