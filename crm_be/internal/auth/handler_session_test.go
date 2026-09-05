package auth_test

// Integration tests (real Postgres via dbtest) for issue #10's endpoints:
// login, refresh rotation + reuse detection, logout, forgot/reset
// password, GET /v1/me, and CSRF. The reuse-detection and FOR UPDATE
// locking guarantees are only meaningful checked against real Postgres —
// TestUnit_Refresh_ReuseDetected_RevokesFamily in usecase_unit_test.go
// proves the business logic branch with a fake; this file is the
// authoritative version.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
)

func doRequest(r *gin.Engine, method, path string, body any, mutate func(*http.Request)) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(req)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func attachCookies(cookies []*http.Cookie) func(*http.Request) {
	return func(req *http.Request) {
		for _, c := range cookies {
			req.AddCookie(c)
		}
	}
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, c := range cookies {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// registerAndVerifyHTTP drives the #9 flow over HTTP so #10's tests
// start from a real, verified account exactly the way a client would.
func registerAndVerifyHTTP(t *testing.T, r *gin.Engine, m *spyMailer, email string) {
	t.Helper()
	w := doJSON(r, http.MethodPost, "/v1/auth/register", map[string]string{
		"organization_name": "Toko ABC",
		"full_name":         "Budi Santoso",
		"email":             email,
		"password":          loginPassword,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	rawToken := m.lastToken(t)
	w = doJSON(r, http.MethodPost, "/v1/auth/verify-email", map[string]string{"token": rawToken})
	if w.Code != http.StatusOK {
		t.Fatalf("verify-email: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Login_Dashboard_SetsCookiesNoTokenInBody(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-login-dashboard@example.com")

	w := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-login-dashboard@example.com", "password": loginPassword, "client": "dashboard",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if bytes.Contains(w.Body.Bytes(), []byte("access_token")) || bytes.Contains(w.Body.Bytes(), []byte("refresh_token")) {
		t.Errorf("dashboard login response must not contain token fields, got: %s", w.Body.String())
	}

	cookies := w.Result().Cookies()
	if cookieValue(cookies, "access_token") == "" {
		t.Error("expected access_token cookie to be set")
	}
	if cookieValue(cookies, "refresh_token") == "" {
		t.Error("expected refresh_token cookie to be set")
	}
	if cookieValue(cookies, httpx.CSRFCookieName) == "" {
		t.Error("expected csrf_token cookie to be set")
	}
}

func TestHandler_Login_Mobile_ReturnsTokenNoCookies(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-login-mobile@example.com")

	w := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-login-mobile@example.com", "password": loginPassword, "client": "mobile",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(w.Result().Cookies()) != 0 {
		t.Errorf("mobile login response must not set any cookies, got: %v", w.Result().Cookies())
	}

	var body struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.AccessToken == "" || body.Data.RefreshToken == "" {
		t.Errorf("expected non-empty access_token and refresh_token in body, got %+v", body.Data)
	}
}

func TestHandler_Login_WrongPassword_Returns401(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-login-wrong@example.com")

	w := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-login-wrong@example.com", "password": "definitely wrong", "client": "dashboard",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Refresh_CookieWithoutCSRFToken_Returns403 is a required
// security test (TD §14): a cookie-authenticated non-GET request without
// X-CSRF-Token must be rejected.
func TestHandler_Refresh_CookieWithoutCSRFToken_Returns403(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-csrf-missing@example.com")

	login := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-csrf-missing@example.com", "password": loginPassword, "client": "dashboard",
	})
	cookies := login.Result().Cookies()

	w := doRequest(r, http.MethodPost, "/v1/auth/refresh", map[string]string{}, attachCookies(cookies))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without X-CSRF-Token, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Refresh_CookieWithCSRFToken_Succeeds is the positive
// counterpart — proves the check isn't just always-reject.
func TestHandler_Refresh_CookieWithCSRFToken_Succeeds(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-csrf-ok@example.com")

	login := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-csrf-ok@example.com", "password": loginPassword, "client": "dashboard",
	})
	cookies := login.Result().Cookies()
	csrf := cookieValue(cookies, httpx.CSRFCookieName)

	w := doRequest(r, http.MethodPost, "/v1/auth/refresh", map[string]string{}, func(req *http.Request) {
		attachCookies(cookies)(req)
		req.Header.Set(httpx.CSRFHeaderName, csrf)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with a matching X-CSRF-Token, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Refresh_MobileBodyToken_NoCSRFRequired is the bearer-side
// counterpart of TD §14's CSRF requirement: a body-sourced refresh token
// (mobile) was never sent automatically by a browser, so it's exempt.
func TestHandler_Refresh_MobileBodyToken_NoCSRFRequired(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-mobile-refresh@example.com")

	login := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-mobile-refresh@example.com", "password": loginPassword, "client": "mobile",
	})
	var body struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	w := doJSON(r, http.MethodPost, "/v1/auth/refresh", map[string]string{"refresh_token": body.Data.RefreshToken})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without X-CSRF-Token for a body-sourced token, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Refresh_Reuse_RevokesFamily is TD §4's acceptance
// criterion, checked against real Postgres so the SELECT ... FOR UPDATE
// rotation (TD §15) is the thing actually under test, not a fake's
// map lookup.
func TestHandler_Refresh_Reuse_RevokesFamily(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-reuse@example.com")

	login := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-reuse@example.com", "password": loginPassword, "client": "mobile",
	})
	var loginBody struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(login.Body.Bytes(), &loginBody)
	originalToken := loginBody.Data.RefreshToken

	rotated := doJSON(r, http.MethodPost, "/v1/auth/refresh", map[string]string{"refresh_token": originalToken})
	if rotated.Code != http.StatusOK {
		t.Fatalf("first refresh: expected 200, got %d: %s", rotated.Code, rotated.Body.String())
	}
	var rotatedBody struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rotated.Body.Bytes(), &rotatedBody)

	// Replay the original (already-rotated) token — reuse detected.
	reuse := doJSON(r, http.MethodPost, "/v1/auth/refresh", map[string]string{"refresh_token": originalToken})
	if reuse.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on reuse, got %d: %s", reuse.Code, reuse.Body.String())
	}

	// The token issued by the rotation must be dead too — whole family revoked.
	afterReuse := doJSON(r, http.MethodPost, "/v1/auth/refresh", map[string]string{"refresh_token": rotatedBody.Data.RefreshToken})
	if afterReuse.Code != http.StatusUnauthorized {
		t.Fatalf("expected the rotated token's family to be revoked too, got %d: %s", afterReuse.Code, afterReuse.Body.String())
	}
}

func TestHandler_Logout_Dashboard_ClearsCookiesAndRevokesToken(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-logout@example.com")

	login := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-logout@example.com", "password": loginPassword, "client": "dashboard",
	})
	cookies := login.Result().Cookies()
	csrf := cookieValue(cookies, httpx.CSRFCookieName)

	w := doRequest(r, http.MethodPost, "/v1/auth/logout", map[string]string{}, func(req *http.Request) {
		attachCookies(cookies)(req)
		req.Header.Set(httpx.CSRFHeaderName, csrf)
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	for _, c := range w.Result().Cookies() {
		if c.Name == "access_token" || c.Name == "refresh_token" {
			if c.Value != "" || c.MaxAge >= 0 {
				t.Errorf("expected %s cookie to be cleared, got value=%q maxAge=%d", c.Name, c.Value, c.MaxAge)
			}
		}
	}
}

func TestHandler_Me_RequiresAuth(t *testing.T) {
	r, _ := newTestRouter(t)

	w := doRequest(r, http.MethodGet, "/v1/me", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Me_WithBearerToken_ReturnsProfile(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-me@example.com")

	login := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-me@example.com", "password": loginPassword, "client": "mobile",
	})
	var body struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(login.Body.Bytes(), &body)

	w := doRequest(r, http.MethodGet, "/v1/me", nil, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+body.Data.AccessToken)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var me struct {
		Data struct {
			Email string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if me.Data.Email != "session-me@example.com" {
		t.Errorf("expected email session-me@example.com, got %q", me.Data.Email)
	}
}

// TestHandler_Me_PlanChannelsKeySetMatchesSubscriptionChannels locks the
// wire contract TD §7 warns about: plan.channels' key set must be
// EXACTLY subscription.Channels. A typo in any of the four places that
// literal is duplicated (planChannels, the three PlanGate call sites,
// this JSON body, and the dashboard's TypeScript) fails silently
// everywhere else — this is the one place it fails loudly.
func TestHandler_Me_PlanChannelsKeySetMatchesSubscriptionChannels(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-plan@example.com")

	login := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-plan@example.com", "password": loginPassword, "client": "mobile",
	})
	var loginBody struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(login.Body.Bytes(), &loginBody)

	w := doRequest(r, http.MethodGet, "/v1/me", nil, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+loginBody.Data.AccessToken)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var me struct {
		Data struct {
			Plan struct {
				Code     string          `json:"code"`
				Channels map[string]bool `json:"channels"`
			} `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if me.Data.Plan.Code != subscription.PlanFree {
		t.Errorf("expected plan.code %q, got %q", subscription.PlanFree, me.Data.Plan.Code)
	}
	if len(me.Data.Plan.Channels) != len(subscription.Channels) {
		t.Fatalf("expected %d keys in plan.channels, got %d: %v", len(subscription.Channels), len(me.Data.Plan.Channels), me.Data.Plan.Channels)
	}
	for _, ch := range subscription.Channels {
		if _, ok := me.Data.Plan.Channels[string(ch)]; !ok {
			t.Errorf("plan.channels missing key %q — must match subscription.Channels exactly", ch)
		}
	}
}

func TestHandler_ForgotPassword_AlwaysReturns202(t *testing.T) {
	r, _ := newTestRouter(t)

	w := doJSON(r, http.MethodPost, "/v1/auth/password/forgot", map[string]string{"email": "nobody-forgot-http@example.com"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 even for an unknown email, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ResetPassword_Success_ThenLoginWithNewPassword(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-reset@example.com")

	w := doJSON(r, http.MethodPost, "/v1/auth/password/forgot", map[string]string{"email": "session-reset@example.com"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("forgot-password: expected 202, got %d: %s", w.Code, w.Body.String())
	}
	rawResetToken := m.lastToken(t)

	w = doJSON(r, http.MethodPost, "/v1/auth/password/reset", map[string]string{
		"token": rawResetToken, "password": "a brand new password 123",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("reset-password: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-reset@example.com", "password": "a brand new password 123", "client": "dashboard",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login with new password: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Me_CarriesPlanLimitsAndUsage is Phase 8.5 §7: the client
// gets ANSWERS (how much is allowed, how much is used), never the
// ingredients to compute them itself — the same rule plan.channels
// already follows.
//
// A freshly registered organization is the sharpest fixture available:
// exactly one membership, zero leads, no invitations. Any number other
// than 1 seat / 0 leads means the meters are reading something other
// than what they claim.
func TestHandler_Me_CarriesPlanLimitsAndUsage(t *testing.T) {
	r, m := newTestRouter(t)
	registerAndVerifyHTTP(t, r, m, "session-usage@example.com")

	login := doJSON(r, http.MethodPost, "/v1/auth/login", map[string]string{
		"email": "session-usage@example.com", "password": loginPassword, "client": "mobile",
	})
	var loginBody struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(login.Body.Bytes(), &loginBody)

	w := doRequest(r, http.MethodGet, "/v1/me", nil, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+loginBody.Data.AccessToken)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var me struct {
		Data struct {
			Plan struct {
				Limits struct {
					LeadsPerMonth int `json:"leads_per_month"`
					Seats         int `json:"seats"`
				} `json:"limits"`
				Usage struct {
					LeadsThisMonth int `json:"leads_this_month"`
					SeatsUsed      int `json:"seats_used"`
				} `json:"usage"`
				TestCheckoutAvailable bool `json:"test_checkout_available"`
			} `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if me.Data.Plan.Limits.LeadsPerMonth <= 0 || me.Data.Plan.Limits.Seats <= 0 {
		t.Errorf("free plan must carry real quantities, got %+v", me.Data.Plan.Limits)
	}
	if me.Data.Plan.Usage.LeadsThisMonth != 0 {
		t.Errorf("a fresh organization has created no leads, got leads_this_month=%d", me.Data.Plan.Usage.LeadsThisMonth)
	}
	if me.Data.Plan.Usage.SeatsUsed != 1 {
		t.Errorf("a fresh organization has exactly its owner, got seats_used=%d", me.Data.Plan.Usage.SeatsUsed)
	}
	// #124 wires the flag and the endpoint it advertises; until then the
	// honest answer is false, and this pins that it isn't accidentally true.
	if me.Data.Plan.TestCheckoutAvailable {
		t.Error("test_checkout_available must be false until #124 wires SUBSCRIPTION_TEST_CHECKOUT")
	}
}
