package task_test

// Integration tests (real Postgres via dbtest) for issue #21's task
// HTTP layer.

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
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/task"
)

const testJWTSecret = "task-handler-test-jwt-secret-32-bytes" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) task.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(task.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(task.Repos{Task: task.New(tx), Activity: activity.NewRecorder(tx)})
	})
}

func (s *testStore) Repos() task.Repos {
	return task.Repos{Task: task.New(s.pool), Activity: activity.NewRecorder(s.pool)}
}

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := task.NewUsecase(newTestStore(pool))

	r := gin.New()
	task.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
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

func extractIDAndVersion(t *testing.T, w *httptest.ResponseRecorder) (string, int) {
	t.Helper()
	var body struct {
		Data struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Data.ID, body.Data.Version
}

func TestHandler_Create_RecordsTaskCreatedActivity(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil)

	w := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/tasks", token, map[string]string{"title": "Follow up"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activities WHERE lead_id = $1 AND type = 'task_created'`, leadID).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 task_created activity, got %d", count)
	}
}

func TestHandler_Complete_SetsFieldsAndRecordsTaskCompleted(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil)

	created := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/tasks", token, map[string]string{"title": "Follow up"})
	id, version := extractIDAndVersion(t, created)

	w := doJSON(r, http.MethodPost, "/v1/tasks/"+id+"/complete", token, map[string]int{"version": version})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			Status                  string  `json:"status"`
			CompletedAt             *string `json:"completed_at"`
			CompletedByMembershipID *string `json:"completed_by_membership_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Status != "done" {
		t.Errorf("expected status done, got %q", body.Data.Status)
	}
	if body.Data.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
	if body.Data.CompletedByMembershipID == nil || *body.Data.CompletedByMembershipID != membershipID.String() {
		t.Errorf("expected completed_by %v, got %v", membershipID, body.Data.CompletedByMembershipID)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activities WHERE lead_id = $1 AND type = 'task_completed'`, leadID).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 task_completed activity, got %d", count)
	}
}

func TestHandler_Update_StaleVersion_Returns409(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil)

	created := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/tasks", token, map[string]string{"title": "Follow up"})
	id, version := extractIDAndVersion(t, created)

	w := doJSON(r, http.MethodPatch, "/v1/tasks/"+id, token, map[string]any{"version": version - 1, "title": "Stale"})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Employee_ReadTaskOnAnotherEmployeesLead_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, ownerUser, ownerMembership := seedOrgAndOwner(t, ctx, pool)
	ownerToken := bearerToken(t, ownerUser, org, ownerMembership, tenant.RoleOwner)

	otherEmployeeUser := uuid.Must(uuid.NewV7())
	otherEmployeeMembership := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Employee')`, otherEmployeeUser, otherEmployeeUser.String()+"@example.com"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'employee')`, otherEmployeeMembership, org, otherEmployeeUser); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	leadID := seedLead(t, ctx, pool, org, &otherEmployeeMembership)

	created := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/tasks", ownerToken, map[string]string{"title": "Follow up"})
	if created.Code != http.StatusCreated {
		t.Fatalf("seed create failed: %d: %s", created.Code, created.Body.String())
	}

	me := uuid.Must(uuid.NewV7())
	myToken := bearerToken(t, me, org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	w := doJSON(r, http.MethodGet, "/v1/leads/"+leadID.String()+"/tasks", myToken, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Employee_DeleteForbidden(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, ownerUser, ownerMembership := seedOrgAndOwner(t, ctx, pool)
	ownerToken := bearerToken(t, ownerUser, org, ownerMembership, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil)

	created := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/tasks", ownerToken, map[string]string{"title": "Follow up"})
	id, _ := extractIDAndVersion(t, created)

	employeeToken := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	w := doJSON(r, http.MethodDelete, "/v1/tasks/"+id, employeeToken, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
