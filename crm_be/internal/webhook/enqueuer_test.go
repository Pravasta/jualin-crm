package webhook_test

// Enqueuer against real Postgres (Phase 7 #101). The selection rules are
// expressed in SQL, so a fake would prove nothing about them — an
// `is_active` predicate or a `= ANY(events)` match either works in the
// database or it doesn't.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

// seedEndpoint inserts one endpoint with explicit control over the three
// fields Enqueue selects on.
func seedEndpoint(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, events []string, active, deleted bool) uuid.UUID {
	t.Helper()
	e := newTestEndpoint()
	e.Events = events
	e.IsActive = active
	if err := webhook.New(pool).Create(ctx, tctx(org), e); err != nil {
		t.Fatalf("seed endpoint: %v", err)
	}
	if deleted {
		if _, err := pool.Exec(ctx, `UPDATE webhook_endpoints SET deleted_at = now() WHERE id = $1`, e.ID); err != nil {
			t.Fatalf("soft delete endpoint: %v", err)
		}
	}
	return e.ID
}

func countDeliveries(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE organization_id = $1`, org).Scan(&n); err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	return n
}

const testPayload = `{"event":"lead.created","data":{"lead":{"id":"x"}}}`

// TestEnqueuer_OneRowPerSubscribedActiveEndpoint is the criterion "two
// endpoints subscribed to lead.created → one lead produces two rows".
func TestEnqueuer_OneRowPerSubscribedActiveEndpoint(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	org := seedOrg(t, ctx, pool)

	first := seedEndpoint(t, ctx, pool, org, []string{webhook.EventLeadCreated}, true, false)
	second := seedEndpoint(t, ctx, pool, org, []string{webhook.EventLeadCreated, webhook.EventLeadStatusChanged}, true, false)

	n, err := webhook.NewEnqueuer(pool).Enqueue(ctx, tctx(org), webhook.EventLeadCreated, []byte(testPayload))
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if n != 2 {
		t.Fatalf("Enqueue wrote %d rows, want 2", n)
	}

	rows, err := pool.Query(ctx, `SELECT endpoint_id, event_type, status, attempt, payload FROM webhook_deliveries WHERE organization_id = $1`, org)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	got := map[uuid.UUID]bool{}
	for rows.Next() {
		var endpointID uuid.UUID
		var eventType, status string
		var attempt int
		var payload []byte
		if err := rows.Scan(&endpointID, &eventType, &status, &attempt, &payload); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[endpointID] = true
		if eventType != webhook.EventLeadCreated {
			t.Errorf("event_type = %q, want %q", eventType, webhook.EventLeadCreated)
		}
		if status != webhook.StatusPending {
			t.Errorf("status = %q, want pending", status)
		}
		if attempt != 0 {
			t.Errorf("attempt = %d, want 0", attempt)
		}
		// The payload is stored verbatim — the frozen snapshot (TD §1.1).
		var decoded, want map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("stored payload is not valid JSON: %v", err)
		}
		if err := json.Unmarshal([]byte(testPayload), &want); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		if decoded["event"] != want["event"] {
			t.Errorf("payload was rewritten: got %v", decoded)
		}
	}
	if !got[first] || !got[second] {
		t.Errorf("expected one row for each endpoint, got %v", got)
	}
}

// TestEnqueuer_SkipsEndpointsThatShouldNotReceive covers the three
// exclusion rules in one table — each is a separate WHERE clause and each
// can regress independently.
func TestEnqueuer_SkipsEndpointsThatShouldNotReceive(t *testing.T) {
	for _, tc := range []struct {
		name    string
		events  []string
		active  bool
		deleted bool
	}{
		{"inactive", []string{webhook.EventLeadCreated}, false, false},
		{"soft deleted", []string{webhook.EventLeadCreated}, true, true},
		{"not subscribed to this event", []string{webhook.EventLeadStatusChanged}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := dbtest.NewPool(t)
			org := seedOrg(t, ctx, pool)
			seedEndpoint(t, ctx, pool, org, tc.events, tc.active, tc.deleted)

			n, err := webhook.NewEnqueuer(pool).Enqueue(ctx, tctx(org), webhook.EventLeadCreated, []byte(testPayload))
			if err != nil {
				t.Fatalf("Enqueue: %v", err)
			}
			if n != 0 {
				t.Errorf("Enqueue wrote %d rows for a %s endpoint, want 0", n, tc.name)
			}
			if got := countDeliveries(t, ctx, pool, org); got != 0 {
				t.Errorf("%d delivery rows exist, want 0", got)
			}
		})
	}
}

// TestEnqueuer_NoEndpointsIsNotAnError is the common case for most
// organizations and must never surface as a failure to the lead create
// that triggered it.
func TestEnqueuer_NoEndpointsIsNotAnError(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	org := seedOrg(t, ctx, pool)

	n, err := webhook.NewEnqueuer(pool).Enqueue(ctx, tctx(org), webhook.EventLeadCreated, []byte(testPayload))
	if err != nil {
		t.Fatalf("Enqueue with no endpoints returned an error: %v", err)
	}
	if n != 0 {
		t.Errorf("wrote %d rows, want 0", n)
	}
}

// TestEnqueuer_NeverCrossesOrganizations is Rule #5/#7 for the one write
// path that is driven by an event rather than a request: another
// organization's endpoint must never receive this organization's lead.
func TestEnqueuer_NeverCrossesOrganizations(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	orgA := seedOrg(t, ctx, pool)
	orgB := seedOrg(t, ctx, pool)

	seedEndpoint(t, ctx, pool, orgB, []string{webhook.EventLeadCreated}, true, false)

	n, err := webhook.NewEnqueuer(pool).Enqueue(ctx, tctx(orgA), webhook.EventLeadCreated, []byte(testPayload))
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if n != 0 {
		t.Errorf("org A's event reached %d of org B's endpoints, want 0", n)
	}
	if got := countDeliveries(t, ctx, pool, orgB); got != 0 {
		t.Errorf("org B has %d delivery rows from org A's event, want 0", got)
	}
}
