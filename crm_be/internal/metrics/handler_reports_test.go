package metrics_test

// Phase 8.6 (#169) — HTTP layer for /trend, /sources, /lost-reasons.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func TestHandler_Trend_WithoutRange_Returns400PerField(t *testing.T) {
	r, pool := newTestRouter(t)
	org := seedOrganization(t, context.Background(), pool)
	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleOwner)

	w := doGet(r, "/v1/metrics/trend", token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Details []struct {
				Field string `json:"field"`
				Code  string `json:"code"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "validation_failed" || len(body.Error.Details) != 2 {
		t.Fatalf("expected validation_failed with from+to details, got %+v", body.Error)
	}
}

func TestHandler_Trend_ReturnsCalendarDates(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	setTimezone(t, ctx, pool, org, "UTC")
	seedLeadAt(t, ctx, pool, org, nil, "new", time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleManager)

	q := url.Values{"from": {"2026-03-01T00:00:00Z"}, "to": {"2026-03-03T23:59:59Z"}}
	w := doGet(r, "/v1/metrics/trend?"+q.Encode(), token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Bucket string `json:"bucket"`
			Points []struct {
				Date  string `json:"date"`
				Count int    `json:"count"`
			} `json:"points"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Bucket != "day" || len(body.Data.Points) != 3 ||
		body.Data.Points[1].Date != "2026-03-02" || body.Data.Points[1].Count != 1 {
		t.Fatalf("expected 3 daily points with 2026-03-02=1 as YYYY-MM-DD, got %+v", body.Data)
	}
}

// An unknown source is ignored (treated as not given), not rejected — same
// rule as every other metrics filter.
func TestHandler_Sources_UnknownSourceIgnored(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	seedLeadAt(t, ctx, pool, org, nil, "new", time.Now().UTC())
	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleAdmin)

	w := doGet(r, "/v1/metrics/sources?source=whatsapp", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data []struct {
			Source string `json:"source"`
			Count  int    `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 4 || body.Data[0].Source != "manual" || body.Data[0].Count != 1 {
		t.Fatalf("expected four sources with manual=1 (filter ignored), got %+v", body.Data)
	}
}

func TestHandler_NewReports_EmployeeForbidden_Returns403(t *testing.T) {
	r, pool := newTestRouter(t)
	org := seedOrganization(t, context.Background(), pool)
	token := bearerToken(t, uuid.Must(uuid.NewV7()), org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)

	q := url.Values{"from": {"2026-03-01T00:00:00Z"}, "to": {"2026-03-03T00:00:00Z"}}.Encode()
	for _, path := range []string{"/v1/metrics/trend?" + q, "/v1/metrics/sources", "/v1/metrics/lost-reasons"} {
		if w := doGet(r, path, token); w.Code != http.StatusForbidden {
			t.Errorf("%s: expected 403 for Employee, got %d", path, w.Code)
		}
	}
}
