package device_test

// Integration tests (real Postgres via dbtest) for issue #68's
// device_token HTTP layer — register/unregister as principal user.
// Nothing here exercises the push-sending side (PushToMembership); that
// has its own coverage in usecase_unit_test.go with a fake Sender.

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

	"github.com/Pravasta/jualin-crm/crm_be/internal/device"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/push"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const testJWTSecret = "device-handler-test-jwt-secret-32bytes" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

type testStore struct{ pool *pgxpool.Pool }

func newTestStore(pool *pgxpool.Pool) device.Store { return &testStore{pool: pool} }

func (s *testStore) InTx(ctx context.Context, fn func(device.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(device.Repos{DeviceToken: device.New(tx)})
	})
}

func (s *testStore) Repos() device.Repos {
	return device.Repos{DeviceToken: device.New(s.pool)}
}

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := device.NewUsecase(newTestStore(pool), push.NewNoopSender(testLogger()), testLogger())

	r := gin.New()
	device.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
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

// --- register ---

func TestHandler_Register_EveryRoleAllowed(t *testing.T) {
	for _, role := range []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin, tenant.RoleManager, tenant.RoleEmployee} {
		r, pool := newTestRouter(t)
		ctx := context.Background()
		org := seedOrganization(t, ctx, pool)
		membershipID := seedMembership(t, ctx, pool, org, "actor-"+string(role)+"@example.com", role)
		token := bearerToken(t, uuid.Must(uuid.NewV7()), org, membershipID, role)

		w := doJSON(r, http.MethodPost, "/v1/device-tokens", token, map[string]string{
			"token": "handler-test-token-" + string(role), "platform": "android",
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("role %s: expected 201, got %d: %s", role, w.Code, w.Body.String())
		}

		var body struct {
			Data struct {
				MembershipID string `json:"membership_id"`
				Platform     string `json:"platform"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Data.MembershipID != membershipID.String() {
			t.Errorf("expected membership_id %s, got %s", membershipID, body.Data.MembershipID)
		}
		if body.Data.Platform != "android" {
			t.Errorf("expected platform android, got %q", body.Data.Platform)
		}
		if bytes.Contains(w.Body.Bytes(), []byte("handler-test-token-"+string(role))) {
			t.Error("expected the response to never echo the raw device token back (Rule #26)")
		}
	}
}

func TestHandler_Register_InvalidPlatform_Returns400(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	membershipID := seedMembership(t, ctx, pool, org, "owner@example.com", tenant.RoleOwner)
	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, membershipID, tenant.RoleOwner)

	w := doJSON(r, http.MethodPost, "/v1/device-tokens", token, map[string]string{
		"token": "tok-1", "platform": "windows-phone",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Register_Unauthenticated_Returns401(t *testing.T) {
	r, _ := newTestRouter(t)
	w := doJSON(r, http.MethodPost, "/v1/device-tokens", "", map[string]string{"token": "tok-1", "platform": "android"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// --- unregister ---

func TestHandler_Unregister_OwnToken_Returns204(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	membershipID := seedMembership(t, ctx, pool, org, "employee@example.com", tenant.RoleEmployee)
	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, membershipID, tenant.RoleEmployee)

	create := doJSON(r, http.MethodPost, "/v1/device-tokens", token, map[string]string{"token": "to-unregister", "platform": "android"})
	if create.Code != http.StatusCreated {
		t.Fatalf("setup: expected 201, got %d: %s", create.Code, create.Body.String())
	}

	del := doJSON(r, http.MethodDelete, "/v1/device-tokens", token, map[string]string{"token": "to-unregister"})
	if del.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", del.Code, del.Body.String())
	}
}

// TestHandler_Unregister_SomeoneElsesToken_Returns404 is the HTTP-level
// proof of device.Usecase.Unregister's ownership check — same org,
// different membership, must come back 404, never 403 (Rule #6's
// reasoning, extended to cross-membership ownership).
func TestHandler_Unregister_SomeoneElsesToken_Returns404(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	victimMembership := seedMembership(t, ctx, pool, org, "victim@example.com", tenant.RoleEmployee)
	victimToken := bearerToken(t, uuid.Must(uuid.NewV7()), org, victimMembership, tenant.RoleEmployee)
	attackerMembership := seedMembership(t, ctx, pool, org, "attacker@example.com", tenant.RoleEmployee)
	attackerToken := bearerToken(t, uuid.Must(uuid.NewV7()), org, attackerMembership, tenant.RoleEmployee)

	create := doJSON(r, http.MethodPost, "/v1/device-tokens", victimToken, map[string]string{"token": "victim-device", "platform": "android"})
	if create.Code != http.StatusCreated {
		t.Fatalf("setup: expected 201, got %d: %s", create.Code, create.Body.String())
	}

	del := doJSON(r, http.MethodDelete, "/v1/device-tokens", attackerToken, map[string]string{"token": "victim-device"})
	if del.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", del.Code, del.Body.String())
	}
}

// seedOrganization/seedMembership are defined once, in repository_test.go
// — shared by every *_test.go file in this package.
