package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

func newCORSRouter(allowedOrigins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(httpx.CORS(allowedOrigins))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCORS_AllowedOrigin_EchoesOriginAndAllowsCredentials(t *testing.T) {
	r := newCORSRouter([]string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("expected Allow-Origin to echo the request origin, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("expected Allow-Credentials=true, got %q", got)
	}
}

func TestCORS_DisallowedOrigin_NoHeadersButRequestStillProcessed(t *testing.T) {
	r := newCORSRouter([]string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Allow-Origin header for a disallowed origin, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("expected no Allow-Credentials header for a disallowed origin, got %q", got)
	}
	// The server never confirms or denies which origins are configured —
	// the browser is what rejects the response, so the handler still runs.
	if w.Code != http.StatusOK {
		t.Errorf("expected the request to still reach the handler (200), got %d", w.Code)
	}
}

func TestCORS_Preflight_Returns204WithoutTouchingHandler(t *testing.T) {
	handlerCalled := false
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(httpx.CORS([]string{"http://localhost:3000"}))
	r.PATCH("/v1/leads/1", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/v1/leads/1", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if handlerCalled {
		t.Error("expected preflight to never reach the route handler")
	}
	if got := w.Header().Get("Access-Control-Max-Age"); got == "" {
		t.Error("expected Access-Control-Max-Age to be set on preflight so PATCH isn't two round trips every time")
	}
}

func TestCORS_NeverSendsWildcardWithCredentials(t *testing.T) {
	r := newCORSRouter([]string{"http://localhost:3000", "https://app.example.com"})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatal("Access-Control-Allow-Origin must never be '*' when credentials are allowed")
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("expected Allow-Credentials=true, got %q", got)
	}
}

func TestCORS_NoOriginHeader_RequestProceedsWithoutCORSHeaders(t *testing.T) {
	r := newCORSRouter([]string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for a same-origin/non-browser request, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Allow-Origin header when Origin is absent, got %q", got)
	}
}
