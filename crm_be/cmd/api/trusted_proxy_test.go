package main

// TestClientIP_* proves the config.TrustedProxies wiring newRouter adds
// in issue #57 actually does something — SetTrustedProxies is dead code
// unless something proves a forged X-Forwarded-For cannot change how a
// real request is rate-limited by IP (Rule #34), and that a genuinely
// configured trusted proxy's header IS honored.
//
// All four Rule #34 call sites (register, verify-email/resend,
// password/forgot, login) are exercised — the leak this issue closes
// touched every one of them, not just one representative endpoint.
//
// Quality bar (freeze bagian 5, lapis 4): provably able to fail.
// Verified manually by removing the r.SetTrustedProxies call from
// newRouter (cmd/api/main.go) and re-running this file — every test
// below went red because ClientIP() started trusting the forged header.
// See docs/phases/04.5-hardening/notes.md's "## #57" section for the
// exact procedure and output; the change itself was never committed.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
)

// noProxyTrustedConfig leaves TrustedProxies at its zero value — the
// same as .env.example's documented default (TRUSTED_PROXIES=none) once
// loaded through config.Load, and what every other router_test.go /
// tenant_isolation_test.go fixture already gets implicitly.
func noProxyTrustedConfig() *config.Config {
	return isolationTestConfig()
}

// trustedProxyConfig trusts the exact /24 httptest.NewRequest's default
// RemoteAddr (192.0.2.1) falls into, so a request actually arriving from
// that address is a "real" proxy newRouter is configured to believe.
func trustedProxyConfig() *config.Config {
	cfg := isolationTestConfig()
	cfg.TrustedProxies = []string{"192.0.2.0/24"}
	return cfg
}

// newRouterWithConfig is newIsolationRouter (tenant_isolation_test.go)
// generalized to accept whichever config.Config a test needs — every
// other caller of newIsolationRouter is fine with isolationTestConfig()
// as-is, so that helper is left untouched.
func newRouterWithConfig(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	return newRouter(testLogger(), pool, cfg)
}

func jsonRequest(method, path string, body any, forgedXFF string) *http.Request {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if forgedXFF != "" {
		req.Header.Set("X-Forwarded-For", forgedXFF)
	}
	return req
}

func serve(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestClientIP_Register_ForgedIPDoesNotBypassRateLimit is Rule #34's
// per-IP half for POST /v1/auth/register. registerLimit is 5/hour
// (internal/auth/handler_http.go) — every one of these 6 calls uses a
// unique email (so the 409 duplicate-email path never intervenes) and a
// DIFFERENT forged X-Forwarded-For each time. If ClientIP() believed the
// header, each call would look like a fresh IP and never trip the limit.
func TestClientIP_Register_ForgedIPDoesNotBypassRateLimit(t *testing.T) {
	r := newRouterWithConfig(t, noProxyTrustedConfig())

	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		req := jsonRequest(http.MethodPost, "/v1/auth/register", map[string]string{
			"organization_name": fmt.Sprintf("Org %d", i),
			"full_name":         "Test User",
			"email":             fmt.Sprintf("proxy-spoof-register-%d@example.com", i),
			"password":          "correct horse battery staple",
		}, fmt.Sprintf("10.0.0.%d", i)) // a different forged client IP on every call
		last = serve(r, req)
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the 6th registration to be rate-limited despite a different forged X-Forwarded-For each time, got %d: %s", last.Code, last.Body.String())
	}
}

// TestClientIP_Resend_ForgedIPDoesNotBypassRateLimit is Rule #34's
// per-IP half for POST /v1/auth/verify-email/resend. resendIPLimit is
// 10/hour; a unique email per call sidesteps the per-email limiter
// (resendLimit=3/hour) entirely, isolating this to the IP side.
func TestClientIP_Resend_ForgedIPDoesNotBypassRateLimit(t *testing.T) {
	r := newRouterWithConfig(t, noProxyTrustedConfig())

	var last *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		req := jsonRequest(http.MethodPost, "/v1/auth/verify-email/resend",
			map[string]string{"email": fmt.Sprintf("proxy-spoof-resend-%d@example.com", i)},
			fmt.Sprintf("10.0.1.%d", i))
		last = serve(r, req)
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the 11th resend to be rate-limited despite a different forged X-Forwarded-For and a fresh email each time, got %d: %s", last.Code, last.Body.String())
	}
}

// TestClientIP_ForgotPassword_ForgedIPDoesNotBypassRateLimit mirrors the
// resend test above for POST /v1/auth/password/forgot (forgotIPLimit is
// also 10/hour).
func TestClientIP_ForgotPassword_ForgedIPDoesNotBypassRateLimit(t *testing.T) {
	r := newRouterWithConfig(t, noProxyTrustedConfig())

	var last *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		req := jsonRequest(http.MethodPost, "/v1/auth/password/forgot",
			map[string]string{"email": fmt.Sprintf("proxy-spoof-forgot-%d@example.com", i)},
			fmt.Sprintf("10.0.2.%d", i))
		last = serve(r, req)
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the 11th forgot-password request to be rate-limited despite a different forged X-Forwarded-For and a fresh email each time, got %d: %s", last.Code, last.Body.String())
	}
}

// TestClientIP_Login_BackoffNotBypassableByForgedIP proves LoginLimiter's
// IP-keyed backoff (TD phase 1 §11) survives a forged header too. One
// failed login records a failure against "login:ip:<real peer>"; an
// immediate second attempt — different forged X-Forwarded-For, different
// (also nonexistent) email so the email-keyed backoff can't be what
// blocks it — must still be rejected, because the real IP is unchanged.
func TestClientIP_Login_BackoffNotBypassableByForgedIP(t *testing.T) {
	r := newRouterWithConfig(t, noProxyTrustedConfig())

	first := serve(r, jsonRequest(http.MethodPost, "/v1/auth/login",
		map[string]string{"email": "nobody-1@example.com", "password": "wrong-password", "client": "dashboard"},
		"10.0.3.1"))
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("expected the first bad login to be 401 (not yet backed off), got %d: %s", first.Code, first.Body.String())
	}

	second := serve(r, jsonRequest(http.MethodPost, "/v1/auth/login",
		map[string]string{"email": "nobody-2@example.com", "password": "wrong-password", "client": "dashboard"},
		"10.0.3.2"))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the second login attempt to be rate-limited by IP backoff despite a different forged X-Forwarded-For and a different email, got %d: %s", second.Code, second.Body.String())
	}
}

// TestClientIP_UntrustedPeerHeaderIgnoredEvenWithProxiesConfigured proves
// a peer OUTSIDE the configured trusted CIDR still can't set its own
// X-Forwarded-For — trusting some proxies must not mean trusting every
// connection.
func TestClientIP_UntrustedPeerHeaderIgnoredEvenWithProxiesConfigured(t *testing.T) {
	r := newRouterWithConfig(t, trustedProxyConfig()) // trusts 192.0.2.0/24 only

	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		req := jsonRequest(http.MethodPost, "/v1/auth/register", map[string]string{
			"organization_name": fmt.Sprintf("Untrusted Org %d", i),
			"full_name":         "Test User",
			"email":             fmt.Sprintf("proxy-untrusted-%d@example.com", i),
			"password":          "correct horse battery staple",
		}, fmt.Sprintf("10.0.4.%d", i))
		req.RemoteAddr = "203.0.113.9:54321" // real peer, deliberately OUTSIDE 192.0.2.0/24
		last = serve(r, req)
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected an untrusted peer's X-Forwarded-For to be ignored (limit still tripped by its real IP), got %d: %s", last.Code, last.Body.String())
	}
}

// TestClientIP_TrustedProxyHeaderIsHonored is the positive half:
// SetTrustedProxies isn't supposed to make every header always ignored,
// only headers from peers it wasn't told to trust. Six calls from a
// trusted proxy address, each declaring a DIFFERENT real client via
// X-Forwarded-For, must be treated as six distinct clients — none of
// them individually reach registerLimit (5/hour).
func TestClientIP_TrustedProxyHeaderIsHonored(t *testing.T) {
	r := newRouterWithConfig(t, trustedProxyConfig()) // trusts 192.0.2.0/24

	for i := 0; i < 6; i++ {
		req := jsonRequest(http.MethodPost, "/v1/auth/register", map[string]string{
			"organization_name": fmt.Sprintf("Trusted Org %d", i),
			"full_name":         "Test User",
			"email":             fmt.Sprintf("proxy-trusted-%d@example.com", i),
			"password":          "correct horse battery staple",
		}, fmt.Sprintf("198.51.100.%d", i)) // six distinct real clients, per the trusted proxy
		req.RemoteAddr = "192.0.2.1:12345" // the trusted proxy itself
		w := serve(r, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("call %d: expected 201 (each forwarded client IP tracked separately), got %d: %s", i, w.Code, w.Body.String())
		}
	}
}
