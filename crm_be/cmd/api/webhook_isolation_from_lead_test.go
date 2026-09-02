package main

// Phase 7 #102's acceptance criterion: "endpoint webhook mati total →
// lead tetap terbuat dan responsnya tetap 201".
//
// The behaviour is already guaranteed by #101's design — enqueuing is a
// database write and nothing on the create path opens a socket — but a
// guarantee that rests on "no one will add an HTTP call here later" is
// worth pinning down. A future change that made delivery synchronous
// would turn one customer's broken receiver into a 500 on every lead the
// whole organization creates, and this test is what would catch it.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func TestWebhookTrigger_DeadEndpointDoesNotBreakLeadCreation(t *testing.T) {
	r, pool := newIsolationRouter(t)
	ctx := t.Context()

	org, _, membershipID := seedOrgOwner(t, pool, "Webhook Org", "owner-webhook@example.com")
	token := mintBearerToken(t, uuid.Must(uuid.NewV7()), org, membershipID, tenant.RoleOwner)

	// Port 1 on loopback: nothing listens there, and nothing can be made
	// to. As dead as a customer's receiver ever gets.
	endpointID := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `
		INSERT INTO webhook_endpoints (id, organization_id, url, secret_ciphertext, secret_prefix, events)
		VALUES ($1, $2, 'http://127.0.0.1:1/definitely-not-listening', $3, 'whsec_dead', ARRAY['lead.created'])`,
		endpointID, org, []byte("sealed-bytes-for-a-dead-endpoint"))
	if err != nil {
		t.Fatalf("seed dead endpoint: %v", err)
	}

	w := doIsolationRequest(r, http.MethodPost, "/v1/leads", token, map[string]any{
		"name":  "Lead With A Dead Webhook",
		"email": "dead-webhook@example.com",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("creating a lead returned %d, want 201 — a customer's broken receiver must never fail their own lead capture: %s",
			w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.ID == "" {
		t.Fatal("response carried no lead id")
	}

	// The lead is real...
	var leads int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE organization_id = $1`, org).Scan(&leads); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if leads != 1 {
		t.Errorf("lead count = %d, want 1", leads)
	}

	// ...and the delivery is queued, waiting for a worker rather than
	// having been attempted inline. Asserting 'pending' is what separates
	// "the write happened" from "the HTTP call happened and failed
	// quietly", which would look identical from the response alone.
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM webhook_deliveries WHERE organization_id = $1`, org).Scan(&status); err != nil {
		t.Fatalf("read delivery: %v", err)
	}
	if status != "pending" {
		t.Errorf("delivery status = %q, want pending — nothing on the request path should have tried to deliver it", status)
	}
}
