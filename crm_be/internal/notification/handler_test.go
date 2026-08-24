package notification_test

// Integration tests (real Postgres via dbtest) for issue #22's
// notification HTTP layer.

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

	"github.com/Pravasta/jualin-crm/crm_be/internal/notification"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const testJWTSecret = "notification-handler-test-jwt-secret-32b" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) notification.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(notification.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(notification.Repos{Notification: notification.New(tx)})
	})
}

func (s *testStore) Repos() notification.Repos {
	return notification.Repos{Notification: notification.New(s.pool)}
}

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := notification.NewUsecase(newTestStore(pool))

	r := gin.New()
	notification.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
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

func seedNotification(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, recipient uuid.UUID, title string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	const q = `INSERT INTO notifications (id, organization_id, recipient_membership_id, type, title) VALUES ($1, $2, $3, 'lead_assigned', $4)`
	if _, err := pool.Exec(ctx, q, id, org, recipient, title); err != nil {
		t.Fatalf("seed notification: %v", err)
	}
	return id
}

func TestHandler_List_UnreadFilter(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	seedNotification(t, ctx, pool, org, membershipID, "hello")

	w := doJSON(r, http.MethodGet, "/v1/notifications", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Data) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(body.Data))
	}

	w2 := doJSON(r, http.MethodPost, "/v1/notifications/"+body.Data[0].ID+"/read", token, nil)
	if w2.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w2.Code, w2.Body.String())
	}

	w3 := doJSON(r, http.MethodGet, "/v1/notifications?unread=true", token, nil)
	var body3 struct {
		Data []any `json:"data"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &body3)
	if len(body3.Data) != 0 {
		t.Errorf("expected 0 unread after marking read, got %d", len(body3.Data))
	}
}

func TestHandler_MarkRead_OtherPersonsNotification_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, _, _ := seedOrgAndOwner(t, ctx, pool)

	ownerUser := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Other')`, ownerUser, ownerUser.String()+"@example.com"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	otherMembership := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'owner')`, otherMembership, org, ownerUser); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	notifID := seedNotification(t, ctx, pool, org, otherMembership, "not yours")

	me := uuid.Must(uuid.NewV7())
	myToken := bearerToken(t, me, org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/notifications/"+notifID.String()+"/read", myToken, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_MarkAllRead(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org, userID, membershipID := seedOrgAndOwner(t, ctx, pool)
	token := bearerToken(t, userID, org, membershipID, tenant.RoleOwner)
	seedNotification(t, ctx, pool, org, membershipID, "one")
	seedNotification(t, ctx, pool, org, membershipID, "two")

	w := doJSON(r, http.MethodPost, "/v1/notifications/read-all", token, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	w2 := doJSON(r, http.MethodGet, "/v1/notifications?unread=true", token, nil)
	var body struct {
		Data []any `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &body)
	if len(body.Data) != 0 {
		t.Errorf("expected 0 unread after read-all, got %d", len(body.Data))
	}
}
