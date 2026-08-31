package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testConfig returns a minimal config sufficient for newRouter — no
// DATABASE_URL is set because these router-level tests never touch the
// database directly.
func testConfig() *config.Config {
	return &config.Config{
		AppEnv:             "development",
		MailProvider:       "log",
		AppBaseURL:         "http://localhost:3000",
		PublicAPIRateLimit: 100,    // see isolationTestConfig's comment (Phase 4 #47)
		PushProvider:       "none", // see isolationTestConfig's comment (Phase 5 #68) — a zero-value "" here would hit newPushSender's unreachable default and panic
		// See isolationTestConfig's comment (Phase 6 #87) — same
		// zero-value-panics-newCaptchaVerifier reasoning as PushProvider.
		CaptchaProvider:         "none",
		FormTokenSecret:         "router-test-form-token-secret-32-bytes",
		FormSubmitRateLimitIP:   100,
		FormSubmitRateLimitForm: 100,
	}
}

func TestHealth_ReturnsOK(t *testing.T) {
	// nil pool is safe here: none of these tests hit /health/ready, and
	// newRouter only closes over the pool — it never dereferences it at
	// build time. /health/ready's DB-down path needs a real database and
	// is covered by the testcontainers harness in issue #3.
	r := newRouter(testLogger(), nil, testConfig())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Data.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", body.Data.Status)
	}
}

func TestUnknownRoute_Returns404WithErrorEnvelope(t *testing.T) {
	// nil pool is safe here: none of these tests hit /health/ready, and
	// newRouter only closes over the pool — it never dereferences it at
	// build time. /health/ready's DB-down path needs a real database and
	// is covered by the testcontainers harness in issue #3.
	r := newRouter(testLogger(), nil, testConfig())
	req := httptest.NewRequest(http.MethodGet, "/this-does-not-exist", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("expected code 'not_found', got %q", body.Error.Code)
	}
}

func TestWrongMethod_Returns405(t *testing.T) {
	// nil pool is safe here: none of these tests hit /health/ready, and
	// newRouter only closes over the pool — it never dereferences it at
	// build time. /health/ready's DB-down path needs a real database and
	// is covered by the testcontainers harness in issue #3.
	r := newRouter(testLogger(), nil, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestPanic_RecoveredAs500WithoutLeakingDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	log := testLogger()
	r.Use(httpx.RequestID(), httpx.Logging(log), httpx.Recovery(log))
	r.GET("/boom", func(c *gin.Context) {
		panic("something exploded")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "something exploded") {
		t.Error("panic message must not leak into the HTTP response")
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("expected code 'internal_error', got %q", body.Error.Code)
	}
}
