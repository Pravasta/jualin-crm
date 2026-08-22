package lead_test

// Integration tests (real Postgres via dbtest) for issue #20's HTTP
// layer. TestHandler_Create_ConcurrentSameIdempotencyKey_ExactlyOneNew
// is the Definition of Done's explicit concurrency requirement — the
// other handler tests could in principle be written against a fake, but
// this one specifically needs the real uq_leads_org_idempotency
// constraint racing two real requests.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const testJWTSecret = "lead-handler-test-jwt-secret-32-bytes" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) lead.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(lead.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(lead.Repos{Lead: lead.New(tx)}) })
}

func (s *testStore) Repos() lead.Repos { return lead.Repos{Lead: lead.New(s.pool)} }

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := lead.NewUsecase(newTestStore(pool))

	r := gin.New()
	lead.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
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

func doJSON(r *gin.Engine, method, path, bearer string, body any, extraHeaders map[string]string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
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

func TestHandler_Create_ReturnsLeadNumberAnd201(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Budi Santoso"}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			LeadNumber int    `json:"lead_number"`
			Name       string `json:"name"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.LeadNumber != 1 {
		t.Errorf("expected lead_number 1, got %d", body.Data.LeadNumber)
	}
	if body.Data.Status != "new" {
		t.Errorf("expected default status new, got %q", body.Data.Status)
	}
}

// TestHandler_Create_ConcurrentSameIdempotencyKey_ExactlyOneNew is the
// Definition of Done's explicit requirement: two concurrent POSTs with
// the same Idempotency-Key must produce exactly one lead, not a race
// between two application-level checks.
// TestHandler_Create_InvalidAssignee_Returns400NotInternalError is a
// regression test for a real bug found while writing THIS issue's own
// tests: assigning a lead to a membership id that doesn't exist used to
// surface as a bare 500 (the fk_leads_assignee violation was unhandled).
func TestHandler_Create_InvalidAssignee_Returns400NotInternalError(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	bogusAssignee := uuid.Must(uuid.NewV7())
	w := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]any{
		"name": "Budi", "assigned_to_membership_id": bogusAssignee.String(),
	}, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a nonexistent assignee, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Create_ConcurrentSameIdempotencyKey_ExactlyOneNew(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	const attempts = 10
	key := "race-key-" + uuid.Must(uuid.NewV7()).String()

	var wg sync.WaitGroup
	codes := make([]int, attempts)
	ids := make([]string, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Race Lead"}, map[string]string{"Idempotency-Key": key})
			codes[idx] = w.Code
			var body struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			ids[idx] = body.Data.ID
		}(i)
	}
	wg.Wait()

	created, replayed := 0, 0
	firstID := ids[0]
	for i, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			replayed++
		default:
			t.Errorf("attempt %d: unexpected status %d", i, code)
		}
		if ids[i] != firstID {
			t.Errorf("attempt %d: expected lead id %q, got %q — replay must return the SAME lead", i, firstID, ids[i])
		}
	}
	if created != 1 {
		t.Errorf("expected exactly 1 created (201), got %d", created)
	}
	if replayed != attempts-1 {
		t.Errorf("expected %d replays (200), got %d", attempts-1, replayed)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE organization_id = $1 AND idempotency_key = $2`, org, key).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row in the database for this idempotency key, got %d", count)
	}
}

func TestHandler_Update_StaleVersion_Returns409WithCurrentState(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Budi"}, nil)
	var createdBody struct {
		Data struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"data"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &createdBody)

	w := doJSON(r, http.MethodPatch, "/v1/leads/"+createdBody.Data.ID, token,
		map[string]any{"version": createdBody.Data.Version - 1, "name": "Stale Update"}, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var conflictBody struct {
		Error struct {
			Code    string `json:"code"`
			Current struct {
				Name string `json:"name"`
			} `json:"current"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &conflictBody); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if conflictBody.Error.Code != "version_conflict" {
		t.Errorf("expected code version_conflict, got %q", conflictBody.Error.Code)
	}
	if conflictBody.Error.Current.Name != "Budi" {
		t.Errorf("expected current.name to reflect the real current state 'Budi', got %q", conflictBody.Error.Current.Name)
	}
}

func TestHandler_UpdateStatus_NewToWon_Returns422(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Budi"}, nil)
	id, version := extractIDAndVersion(t, created)

	w := doJSON(r, http.MethodPatch, "/v1/leads/"+id+"/status", token, map[string]any{"version": version, "status": "won"}, nil)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateStatus_LostWithoutReason_Rejected(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	created := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Budi"}, nil)
	id, version := extractIDAndVersion(t, created)

	w := doJSON(r, http.MethodPatch, "/v1/leads/"+id+"/status", token, map[string]any{"version": version, "status": "lost"}, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for lost without a reason, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Create_PhoneNormalization(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Budi", "phone": "0812-3456-7890"}, nil)
	var body struct {
		Data struct {
			Phone     string `json:"phone"`
			PhoneE164 string `json:"phone_e164"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Data.PhoneE164 != "+6281234567890" {
		t.Errorf("expected phone_e164 +6281234567890, got %q", body.Data.PhoneE164)
	}

	w2 := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Unparseable", "phone": "1234"}, nil)
	var body2 struct {
		Data struct {
			PhoneE164 *string `json:"phone_e164"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &body2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("expected an unparseable phone to still be accepted (201), got %d: %s", w2.Code, w2.Body.String())
	}
	if body2.Data.PhoneE164 != nil {
		t.Errorf("expected phone_e164 null for an unparseable number, got %v", *body2.Data.PhoneE164)
	}
}

func TestHandler_Create_EmployeeForbidden(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, _, _ := seedOrgAndOwner(t, ctx, pool)
	employeeUserID := uuid.Must(uuid.NewV7())
	employeeMembershipID := uuid.Must(uuid.NewV7())
	token := bearerToken(t, employeeUserID, org, employeeMembershipID, tenant.RoleEmployee)

	w := doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Budi"}, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Get_Employee_OtherPersonsLead_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	ownerToken := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	// The assignee must be a real membership row — leads.assigned_to_membership_id
	// carries a composite FK (Rule #3); a random UUID would fail the INSERT.
	otherEmployeeMembership := seedEmployeeMembership(t, ctx, pool, org)
	created := doJSON(r, http.MethodPost, "/v1/leads", ownerToken, map[string]any{
		"name": "Assigned To Someone Else", "assigned_to_membership_id": otherEmployeeMembership.String(),
	}, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("seed create failed: %d: %s", created.Code, created.Body.String())
	}
	id, _ := extractIDAndVersion(t, created)

	me := uuid.Must(uuid.NewV7())
	myToken := bearerToken(t, me, org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	w := doJSON(r, http.MethodGet, "/v1/leads/"+id, myToken, nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
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

func TestHandler_List_FilterAssignedToNone(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)

	doJSON(r, http.MethodPost, "/v1/leads", token, map[string]string{"name": "Unassigned"}, nil)
	doJSON(r, http.MethodPost, "/v1/leads", token, map[string]any{"name": "Assigned", "assigned_to_membership_id": membershipID.String()}, nil)

	w := doJSON(r, http.MethodGet, "/v1/leads?assigned_to=none", token, nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Meta.Total != 1 || len(body.Data) != 1 || body.Data[0].Name != "Unassigned" {
		t.Errorf("expected exactly 1 unassigned lead, got %+v", body)
	}
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
