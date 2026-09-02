package lead

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Event names this package emits. Duplicated as string literals rather
// than imported from internal/webhook — that import is exactly what
// WebhookEnqueuer's consumer-declared interface exists to avoid
// (ADR-011). webhook.EventLeadCreated / EventLeadStatusChanged hold the
// same two values, and TestWebhookPayload_EventNamesMatchWebhookPackage
// fails if the two ever drift.
const (
	eventLeadCreated       = "lead.created"
	eventLeadStatusChanged = "lead.status_changed"
)

// webhookPayload is the frozen event snapshot stored on every delivery
// row (TD §5). It carries no delivery_id: one snapshot is shared by every
// endpoint subscribed to the event, and each delivery row's own id serves
// as the delivery_id the worker injects at send time (#102).
//
// occurred_at is the moment the event happened, not the moment it is
// delivered — a retry six hours later still reports when the lead was
// actually created.
type webhookPayload struct {
	Event          string         `json:"event"`
	OccurredAt     time.Time      `json:"occurred_at"`
	OrganizationID string         `json:"organization_id"`
	Data           map[string]any `json:"data"`
	Changes        map[string]any `json:"changes,omitempty"`
}

// buildLeadEvent serializes one lead event. data.lead is Lead.Fields —
// the same shape the dashboard API returns, never a second one.
func buildLeadEvent(t tenant.Context, event string, l *Lead, changes map[string]any) ([]byte, error) {
	p := webhookPayload{
		Event:          event,
		OccurredAt:     time.Now().UTC(),
		OrganizationID: t.OrganizationID.String(),
		Data:           map[string]any{"lead": l.Fields()},
		Changes:        changes,
	}
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("lead: build %s payload: %w", event, err)
	}
	return body, nil
}

// enqueueWebhook is called from INSIDE the caller's transaction, so a
// failure here aborts the whole operation (TD §5). That is intentional
// and is the acceptance criterion #101 states: a lead must never commit
// without the deliveries that announce it, and vice versa.
//
// A nil enqueuer is a no-op rather than an error. Repos.Webhook is nil in
// the many existing tests in this package that predate webhooks and have
// no opinion about them; the composition root always supplies a real one.
func enqueueWebhook(ctx context.Context, r Repos, t tenant.Context, event string, l *Lead, changes map[string]any) error {
	if r.Webhook == nil {
		return nil
	}
	body, err := buildLeadEvent(t, event, l, changes)
	if err != nil {
		return err
	}
	if _, err := r.Webhook.Enqueue(ctx, t, event, body); err != nil {
		return err
	}
	return nil
}
