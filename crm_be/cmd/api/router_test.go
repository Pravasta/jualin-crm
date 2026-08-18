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

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHealth_ReturnsOK(t *testing.T) {
	r := newRouter(testLogger())
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
	r := newRouter(testLogger())
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
	r := newRouter(testLogger())
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
