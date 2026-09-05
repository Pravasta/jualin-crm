package auth_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
)

func newTestRouter(t *testing.T) (*gin.Engine, *spyMailer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	m := &spyMailer{}
	svc := auth.NewUsecase(newTestStore(pool), m, testLogger(), "http://localhost:3000", testTokenConfig())

	r := gin.New()
	auth.NewHandler(svc, auth.CookieConfig{Domain: "", Secure: false}, auth.MeConfig{}).RegisterRoutes(r, authn.Middleware(svc))
	return r, m
}

func doJSON(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_Register_ReturnsCreated(t *testing.T) {
	r, _ := newTestRouter(t)

	w := doJSON(r, http.MethodPost, "/v1/auth/register", map[string]string{
		"organization_name": "Toko ABC",
		"full_name":         "Budi Santoso",
		"email":             "handler@example.com",
		"password":          "correct horse battery staple",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			UserID         string `json:"user_id"`
			OrganizationID string `json:"organization_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Data.UserID == "" || body.Data.OrganizationID == "" {
		t.Errorf("expected non-empty user_id and organization_id, got %+v", body.Data)
	}
}

func TestHandler_Register_DuplicateEmail_Returns409(t *testing.T) {
	r, _ := newTestRouter(t)

	payload := map[string]string{
		"organization_name": "Toko ABC",
		"full_name":         "Budi Santoso",
		"email":             "dup-handler@example.com",
		"password":          "correct horse battery staple",
	}
	doJSON(r, http.MethodPost, "/v1/auth/register", payload)
	w := doJSON(r, http.MethodPost, "/v1/auth/register", payload)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error.Code != "email_already_registered" {
		t.Errorf("expected code email_already_registered, got %q", body.Error.Code)
	}
}

func TestHandler_VerifyEmail_InvalidToken_Returns400(t *testing.T) {
	r, _ := newTestRouter(t)

	w := doJSON(r, http.MethodPost, "/v1/auth/verify-email", map[string]string{"token": "nonsense"})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ResendVerification_AlwaysReturns202(t *testing.T) {
	r, _ := newTestRouter(t)

	// Unknown email — must still be 202 (anti-enumeration, TD §6.3).
	w := doJSON(r, http.MethodPost, "/v1/auth/verify-email/resend", map[string]string{
		"email": "nobody-handler@example.com",
	})

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 even for an unknown email, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandler_Register_RateLimited proves the rate limit is actually
// enforced at the HTTP layer, not just present in code — the acceptance
// criterion is "rate limit terbukti aktif di test".
func TestHandler_Register_RateLimited(t *testing.T) {
	r, _ := newTestRouter(t)

	// httptest.NewRequest defaults RemoteAddr to the same value across
	// calls, so every request in this test shares one rate-limit key.
	// This only proves the limiter itself works when its key doesn't
	// change — it does NOT prove the key can't be forged, because
	// newTestRouter above builds a bare gin.New() with no
	// SetTrustedProxies call, same as any router without it (Gin's
	// default trusts every peer as a proxy). That proof needs the real
	// production router (cmd/api's newRouter, wired in issue #57) and
	// lives in cmd/api/trusted_proxy_test.go instead.
	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		last = doJSON(r, http.MethodPost, "/v1/auth/register", map[string]string{
			"organization_name": fmt.Sprintf("Org %d", i),
			"full_name":         "Test User",
			"email":             fmt.Sprintf("ratelimit-%d@example.com", i),
			"password":          "correct horse battery staple",
		})
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the 6th registration from the same IP within the window to be rate limited (429), got %d: %s", last.Code, last.Body.String())
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(last.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error.Code != "rate_limited" {
		t.Errorf("expected code rate_limited, got %q", body.Error.Code)
	}
}

// TestHandler_ResendVerification_RateLimitedPerEmail proves the
// per-email dimension specifically — repeatedly resending for the SAME
// email must eventually be limited even though each request could come
// from a different IP in principle.
func TestHandler_ResendVerification_RateLimitedPerEmail(t *testing.T) {
	r, _ := newTestRouter(t)

	var last *httptest.ResponseRecorder
	for i := 0; i < 4; i++ {
		last = doJSON(r, http.MethodPost, "/v1/auth/verify-email/resend", map[string]string{
			"email": "same-email@example.com",
		})
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the 4th resend for the same email within the window to be rate limited, got %d: %s", last.Code, last.Body.String())
	}
}
