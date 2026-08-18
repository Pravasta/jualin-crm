package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
)

func TestHealthReady_ReturnsOK_WhenDatabaseReachable(t *testing.T) {
	pool := dbtest.NewPool(t)

	r := newRouter(testLogger(), pool, testConfig())
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			Status   string `json:"status"`
			Database string `json:"database"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Data.Status != "ok" || body.Data.Database != "reachable" {
		t.Errorf("expected status=ok database=reachable, got %+v", body.Data)
	}
}

func TestHealthReady_Returns503_WhenDatabaseUnreachable(t *testing.T) {
	// A pool pointed at a closed local port fails fast without needing a
	// real container teardown — Ping has nothing to connect to.
	pool, err := pgxpool.New(t.Context(), "postgres://jualin:jualin@127.0.0.1:1/jualin_crm_test?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("failed to construct pool: %v", err)
	}
	t.Cleanup(pool.Close)

	r := newRouter(testLogger(), pool, testConfig())
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			Status   string `json:"status"`
			Database string `json:"database"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Data.Status != "degraded" || body.Data.Database != "unreachable" {
		t.Errorf("expected status=degraded database=unreachable, got %+v", body.Data)
	}
}

func TestHealth_UnaffectedByDatabaseState(t *testing.T) {
	// /health must stay 200 even with a pool that can never connect — it
	// is liveness, not readiness, and must never touch the database.
	pool, err := pgxpool.New(t.Context(), "postgres://jualin:jualin@127.0.0.1:1/jualin_crm_test?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("failed to construct pool: %v", err)
	}
	t.Cleanup(pool.Close)

	r := newRouter(testLogger(), pool, testConfig())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected /health to stay 200 regardless of DB state, got %d", w.Code)
	}
}
