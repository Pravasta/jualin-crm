package auth_test

// Issue #136 against real Postgres and the real router: an Employee uses the
// mobile app, not the dashboard. usecase_unit_test.go proves the branches with
// a fake; what only a database can prove is what THIS file checks — that a
// role changed by a plain UPDATE (not by any endpoint of ours) still ends a
// dashboard session, that the refusal leaves no cookie behind, and that it
// does not poison the login limiter mobile shares.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

// newGateRouter is newTestRouter plus the pool: these tests change a role
// with SQL, deliberately bypassing membership.Usecase.UpdateRole — the gate
// must hold for ANY way a role changes, not just the endpoint we wrote.
func newGateRouter(t *testing.T) (*gin.Engine, *spyMailer, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	r := gin.New()
	auth.NewHandler(svc, auth.CookieConfig{Domain: "", Secure: false}, auth.MeConfig{}).RegisterRoutes(r, authn.Middleware(svc))
	return r, m, pool
}

func makeEmployee(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	tag, err := pool.Exec(context.Background(),
		`UPDATE memberships SET role = 'employee' WHERE user_id = (SELECT id FROM users WHERE email = $1)`, email)
	if err != nil {
		t.Fatalf("demote to employee: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("expected exactly 1 membership demoted, got %d", tag.RowsAffected())
	}
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error envelope: %v (%s)", err, body)
	}
	return env.Error.Code
}

func TestHandler_Login_Employee_Dashboard_Returns403_WithoutAnySessionCookie(t *testing.T) {
	r, m, pool := newGateRouter(t)
	registerAndVerifyHTTP(t, r, m, "gate-employee@example.com")
	makeEmployee(t, pool, "gate-employee@example.com")

	w := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "gate-employee@example.com", "password": loginPassword, "client": "dashboard",
	})

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if got := errorCode(t, w.Body.Bytes()); got != "dashboard_not_available_for_role" {
		t.Errorf("expected dashboard_not_available_for_role, got %q", got)
	}
	// Nothing to fall back on: no access_token, no refresh_token, no CSRF
	// cookie may exist for a login that was refused.
	if cookies := w.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("a refused login must set no cookies, got %v", cookies)
	}
}

// The refusal happens AFTER the password verified, and the limiter key
// "login:email:<email>" is shared with mobile. Counting it as a failure would
// let an Employee who keeps opening the dashboard get 429 on the one client
// they ARE allowed to use. Backoff starts at one second, so a SECOND
// immediate attempt is enough to tell the difference.
func TestHandler_Login_EmployeeRefusals_DoNotThrottleTheirMobileLogin(t *testing.T) {
	r, m, pool := newGateRouter(t)
	registerAndVerifyHTTP(t, r, m, "gate-limiter@example.com")
	makeEmployee(t, pool, "gate-limiter@example.com")

	for i := 1; i <= 4; i++ {
		w := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
			"email": "gate-limiter@example.com", "password": loginPassword, "client": "dashboard",
		})
		if w.Code != http.StatusForbidden {
			t.Fatalf("attempt %d: expected 403 every time (never 429), got %d: %s", i, w.Code, w.Body.String())
		}
	}

	w := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "gate-limiter@example.com", "password": loginPassword, "client": "mobile",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("mobile login must be unaffected, got %d: %s", w.Code, w.Body.String())
	}
}

// The wrong-password path must STILL feed the limiter — the exemption is for
// the one refusal that follows a verified password, not for login failures.
func TestHandler_Login_WrongPassword_StillFeedsTheLimiter(t *testing.T) {
	r, m, _ := newGateRouter(t)
	registerAndVerifyHTTP(t, r, m, "gate-still-limited@example.com")

	first := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "gate-still-limited@example.com", "password": "wrong password entirely", "client": "dashboard",
	})
	second := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "gate-still-limited@example.com", "password": "wrong password entirely", "client": "dashboard",
	})

	if first.Code != http.StatusUnauthorized {
		t.Fatalf("first wrong password: expected 401, got %d", first.Code)
	}
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second immediate wrong password: expected 429 (backoff), got %d", second.Code)
	}
}

// TestHandler_Refresh_DashboardSessionEndsWhenRoleBecomesEmployee is the
// acceptance criterion the whole issue turns on, and it is deliberately NOT
// driven through PATCH /v1/memberships/{id}: the gate lives in Refresh, so it
// must hold however the role changed, including sessions that were opened
// before the gate existed.
func TestHandler_Refresh_DashboardSessionEndsWhenRoleBecomesEmployee_MobileSurvives(t *testing.T) {
	r, m, pool := newGateRouter(t)
	registerAndVerifyHTTP(t, r, m, "gate-refresh@example.com")

	dashboard := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "gate-refresh@example.com", "password": loginPassword, "client": "dashboard",
	})
	if dashboard.Code != http.StatusOK {
		t.Fatalf("owner dashboard login: expected 200, got %d: %s", dashboard.Code, dashboard.Body.String())
	}
	cookies := dashboard.Result().Cookies()
	csrf := cookieValue(cookies, httpx.CSRFCookieName)

	mobile := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "gate-refresh@example.com", "password": loginPassword, "client": "mobile",
	})
	var mobileBody struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(mobile.Body.Bytes(), &mobileBody); err != nil || mobileBody.Data.RefreshToken == "" {
		t.Fatalf("mobile login must return a refresh token: %v (%s)", err, mobile.Body.String())
	}

	makeEmployee(t, pool, "gate-refresh@example.com")

	// Dashboard: the session opened as Owner can no longer renew itself.
	w := doRequest(r, http.MethodPost, "/v1/auth/refresh", map[string]string{}, func(req *http.Request) {
		attachCookies(cookies)(req)
		req.Header.Set(httpx.CSRFHeaderName, csrf)
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("dashboard refresh after demotion: expected 401, got %d: %s", w.Code, w.Body.String())
	}

	// ...and its tokens are revoked in the database, not merely refused once.
	var live int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM refresh_tokens WHERE client = 'dashboard' AND revoked_at IS NULL`).Scan(&live); err != nil {
		t.Fatalf("count live dashboard tokens: %v", err)
	}
	if live != 0 {
		t.Errorf("expected every dashboard refresh token revoked, %d still live", live)
	}

	// Mobile: same membership, same new role — untouched.
	mw := doJSON(r, http.MethodPost, "/v1/auth/refresh", map[string]string{"refresh_token": mobileBody.Data.RefreshToken})
	if mw.Code != http.StatusOK {
		t.Fatalf("mobile refresh must still work for an Employee, got %d: %s", mw.Code, mw.Body.String())
	}
}
