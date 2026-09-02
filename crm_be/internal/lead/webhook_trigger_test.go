package lead_test

// The lead → outbound-webhook trigger (Phase 7 #101, TD §5). These are
// fake-Store tests: they prove the Go-level wiring — which event fires,
// what the payload contains, and that a queue failure aborts the
// operation. That a real Postgres transaction actually rolls the lead
// back alongside it is proven separately in repository_atomicity_test.go,
// the same two-layer discipline #21 used for activity.

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

// ownerActorCtx is a one-line wrapper over the shared ownerActor helper —
// these tests never need the organization id it also returns.
func ownerActorCtx() tenant.Context {
	ctx, _ := ownerActor()
	return ctx
}

func strPtr(s string) *string { return &s }

// decodeEvent parses one enqueued payload.
func decodeEvent(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("enqueued payload is not valid JSON: %v\n%s", err, raw)
	}
	return m
}

// TestUnit_EventNamesMatchWebhookPackage is the compiler check ADR-011
// costs us. internal/lead declares its event names as plain strings so it
// never has to import internal/webhook; this test — which, being in
// lead_test, may import both — is what keeps the two copies in step.
//
// Without it, renaming webhook.EventLeadCreated would leave lead happily
// enqueuing an event no endpoint can ever subscribe to, and nothing would
// fail until someone noticed deliveries had stopped.
func TestUnit_EventNamesMatchWebhookPackage(t *testing.T) {
	s := newFakeStore()
	u := lead.NewUsecase(s)

	if _, _, err := u.Create(context.Background(), ownerActorCtx(), lead.CreateLeadInput{Name: "Event Name"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(s.webhook.calls) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(s.webhook.calls))
	}
	if got := s.webhook.calls[0].event; got != webhook.EventLeadCreated {
		t.Errorf("lead enqueued %q, but webhook.EventLeadCreated is %q — the two copies have drifted", got, webhook.EventLeadCreated)
	}
}

func TestUnit_Create_EnqueuesLeadCreated(t *testing.T) {
	s := newFakeStore()
	u := lead.NewUsecase(s)
	ctx := ownerActorCtx()

	created, _, err := u.Create(context.Background(), ctx, lead.CreateLeadInput{
		Name:  "Webhook Trigger",
		Email: strPtr("trigger@example.com"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(s.webhook.calls) != 1 {
		t.Fatalf("expected exactly 1 enqueue, got %d", len(s.webhook.calls))
	}

	payload := decodeEvent(t, s.webhook.calls[0].payload)
	if payload["event"] != webhook.EventLeadCreated {
		t.Errorf("event = %v, want %s", payload["event"], webhook.EventLeadCreated)
	}
	if payload["organization_id"] != ctx.OrganizationID.String() {
		t.Errorf("organization_id = %v, want %s", payload["organization_id"], ctx.OrganizationID)
	}
	if payload["occurred_at"] == nil || payload["occurred_at"] == "" {
		t.Error("occurred_at is missing")
	}
	// lead.created carries no changes block.
	if _, present := payload["changes"]; present {
		t.Errorf("lead.created should not carry a changes block, got %v", payload["changes"])
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is not an object: %v", payload["data"])
	}
	leadObj, ok := data["lead"].(map[string]any)
	if !ok {
		t.Fatalf("data.lead is not an object: %v", data["lead"])
	}
	if leadObj["id"] != created.ID.String() {
		t.Errorf("data.lead.id = %v, want %s", leadObj["id"], created.ID)
	}
	if leadObj["name"] != "Webhook Trigger" {
		t.Errorf("data.lead.name = %v, want Webhook Trigger", leadObj["name"])
	}
}

// TestUnit_Create_PayloadIsTheSameShapeAsTheAPI is TD §5's "data.lead
// memakai leadJSON yang sama, bukan bentuk kedua". Asserting the exact
// key set is what makes a divergence fail here rather than silently ship
// a webhook body that has drifted from the dashboard's lead.
func TestUnit_Create_PayloadIsTheSameShapeAsTheAPI(t *testing.T) {
	s := newFakeStore()
	u := lead.NewUsecase(s)

	created, _, err := u.Create(context.Background(), ownerActorCtx(), lead.CreateLeadInput{Name: "Shape Check"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	payload := decodeEvent(t, s.webhook.calls[0].payload)
	got := payload["data"].(map[string]any)["lead"].(map[string]any)

	for key := range created.Fields() {
		if _, present := got[key]; !present {
			t.Errorf("data.lead is missing %q, which the API's lead shape has", key)
		}
	}
	for key := range got {
		if _, present := created.Fields()[key]; !present {
			t.Errorf("data.lead has extra key %q the API's lead shape does not", key)
		}
	}
}

func TestUnit_UpdateStatus_EnqueuesStatusChangedWithChanges(t *testing.T) {
	s := newFakeStore()
	u := lead.NewUsecase(s)
	ctx := ownerActorCtx()

	created, _, err := u.Create(context.Background(), ctx, lead.CreateLeadInput{Name: "Status Change"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.webhook.calls = nil // drop the lead.created from the fixture

	if _, err := u.UpdateStatus(context.Background(), ctx, created.ID, lead.UpdateStatusInput{
		Status:  "contacted",
		Version: created.Version,
	}); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	if len(s.webhook.calls) != 1 {
		t.Fatalf("expected exactly 1 enqueue, got %d", len(s.webhook.calls))
	}
	if got := s.webhook.calls[0].event; got != webhook.EventLeadStatusChanged {
		t.Fatalf("event = %q, want %q", got, webhook.EventLeadStatusChanged)
	}

	payload := decodeEvent(t, s.webhook.calls[0].payload)
	changes, ok := payload["changes"].(map[string]any)
	if !ok {
		t.Fatalf("changes is not an object: %v", payload["changes"])
	}
	status, ok := changes["status"].(map[string]any)
	if !ok {
		t.Fatalf("changes.status is not an object: %v", changes["status"])
	}
	// The same two keys activities.metadata uses for status_changed
	// (#21) — deliberately not a third representation.
	if status["from"] != "new" {
		t.Errorf("changes.status.from = %v, want %s", status["from"], "new")
	}
	if status["to"] != "contacted" {
		t.Errorf("changes.status.to = %v, want %s", status["to"], "contacted")
	}

	// The lead body must reflect the NEW status, not the one it left.
	leadObj := payload["data"].(map[string]any)["lead"].(map[string]any)
	if leadObj["status"] != "contacted" {
		t.Errorf("data.lead.status = %v, want the post-change status %s", leadObj["status"], "contacted")
	}
}

// TestUnit_Create_EnqueueFailureAbortsTheLead is the acceptance criterion
// at the Go level: the enqueue runs inside InTx, so its failure has to
// propagate out of Create rather than be swallowed. If this ever starts
// passing with a nil error, a lead is committing without the deliveries
// that announce it and the event is lost forever.
func TestUnit_Create_EnqueueFailureAbortsTheLead(t *testing.T) {
	s := newFakeStore()
	s.webhook.err = errors.New("webhook: enqueue failed")
	u := lead.NewUsecase(s)

	_, _, err := u.Create(context.Background(), ownerActorCtx(), lead.CreateLeadInput{Name: "Should Fail"})
	if err == nil {
		t.Fatal("expected the enqueue failure to abort Create, got nil")
	}
}

func TestUnit_UpdateStatus_EnqueueFailureAbortsTheChange(t *testing.T) {
	s := newFakeStore()
	u := lead.NewUsecase(s)
	ctx := ownerActorCtx()

	created, _, err := u.Create(context.Background(), ctx, lead.CreateLeadInput{Name: "Abort Status"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	s.webhook.err = errors.New("webhook: enqueue failed")
	if _, err := u.UpdateStatus(context.Background(), ctx, created.ID, lead.UpdateStatusInput{
		Status:  "contacted",
		Version: created.Version,
	}); err == nil {
		t.Fatal("expected the enqueue failure to abort UpdateStatus, got nil")
	}
}

// TestUnit_NilEnqueuerIsANoOp guards the nil tolerance Repos.Webhook
// documents — the many tests in this package that predate webhooks
// construct Repos without one and must keep working.
func TestUnit_NilEnqueuerIsANoOp(t *testing.T) {
	s := newFakeStore()
	s.repos.Webhook = nil
	u := lead.NewUsecase(s)

	if _, _, err := u.Create(context.Background(), ownerActorCtx(), lead.CreateLeadInput{Name: "No Webhooks"}); err != nil {
		t.Fatalf("Create with a nil enqueuer should succeed, got %v", err)
	}
}
