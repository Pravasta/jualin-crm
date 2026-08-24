package activity_test

// Integration tests (real Postgres via dbtest) for issue #21's activity
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
)

const testJWTSecret = "activity-handler-test-jwt-secret-32b" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) activity.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(activity.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(activity.Repos{Activity: activity.New(tx)}) })
}

func (s *testStore) Repos() activity.Repos { return activity.Repos{Activity: activity.New(s.pool)} }

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := activity.NewUsecase(newTestStore(pool))

	r := gin.New()
	activity.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
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

// seedLead is defined in repository_test.go (same package) and reused
// here.

func TestHandler_Create_NoteAdded_Returns201(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil)

	w := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/activities", token, map[string]string{"type": "note_added", "body": "Called, no answer"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Create_SystemType_Returns422 is TD §8/#21's explicit
// requirement: a client submitting a system-generated type is rejected
// 422, never silently accepted.
func TestHandler_Create_SystemType_Returns422(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil)

	w := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/activities", token, map[string]string{"type": "status_changed"})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for a client-submitted system type, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Get_Employee_OtherPersonsLead_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, _, _ := seedOrgAndOwner(t, ctx, pool)
	otherEmployee := seedEmployeeMembership(t, ctx, pool, org)
	leadID := seedLead(t, ctx, pool, org, &otherEmployee)

	me := uuid.Must(uuid.NewV7())
	myToken := bearerToken(t, me, org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	w := doJSON(r, http.MethodGet, "/v1/leads/"+leadID.String()+"/activities", myToken, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Create_Employee_OtherPersonsLead_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, _, _ := seedOrgAndOwner(t, ctx, pool)
	otherEmployee := seedEmployeeMembership(t, ctx, pool, org)
	leadID := seedLead(t, ctx, pool, org, &otherEmployee)

	me := uuid.Must(uuid.NewV7())
	myToken := bearerToken(t, me, org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	w := doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/activities", myToken, map[string]string{"type": "note_added"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_RoutesAreAppendOnly checks the actual registered route
// list — not assumption — for exactly GET and POST on
// /v1/leads/:id/activities, per TD §8's explicit "no PATCH/DELETE"
// requirement and this issue's acceptance criteria ("diperiksa lewat
// daftar route, bukan asumsi").
func TestHandler_RoutesAreAppendOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	u := activity.NewUsecase(newTestStore(nil))
	activity.NewHandler(u).RegisterRoutes(r, func(c *gin.Context) { c.Next() })

	methods := map[string]bool{}
	for _, route := range r.Routes() {
		if route.Path == "/v1/leads/:id/activities" {
			methods[route.Method] = true
		}
	}
	if !methods[http.MethodGet] || !methods[http.MethodPost] {
		t.Fatalf("expected GET and POST registered on /v1/leads/:id/activities, got %v", methods)
	}
	if methods[http.MethodPatch] || methods[http.MethodDelete] || methods[http.MethodPut] {
		t.Fatalf("expected NO PATCH/DELETE/PUT on /v1/leads/:id/activities — append-only, got %v", methods)
	}
	if len(methods) != 2 {
		t.Fatalf("expected exactly 2 methods registered, got %v", methods)
	}
}

func TestHandler_List_NewestFirst(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	leadID := seedLead(t, ctx, pool, org, nil)

	doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/activities", token, map[string]string{"type": "note_added", "body": "first"})
	doJSON(r, http.MethodPost, "/v1/leads/"+leadID.String()+"/activities", token, map[string]string{"type": "note_added", "body": "second"})

	w := doJSON(r, http.MethodGet, "/v1/leads/"+leadID.String()+"/activities", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data []struct {
			Body string `json:"body"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 2 || body.Data[0].Body != "second" || body.Data[1].Body != "first" {
		t.Fatalf("expected newest-first ordering, got %+v", body.Data)
	}
}
