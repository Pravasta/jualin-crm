package lead_test

// TestCreate_ActivityRecordFailureInsideInTx_RollsBackLead is issue
// #21's atomicity acceptance criterion proven against a REAL
// transaction, not just usecase_unit_test.go's fake — the fake proves
// the Go-level wiring returns an error; this proves the database
// actually rolls back the lead insert alongside it. Same "prove it
// against real Postgres, not just a fake" discipline as
// repository_test.go's TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

// failingRecorder satisfies lead.ActivityRecorder and always fails —
// standing in for a real activity write that goes wrong mid-transaction.
type failingRecorder struct{}

func (failingRecorder) Record(context.Context, tenant.Context, uuid.UUID, string, *uuid.UUID, map[string]any) error {
	return errors.New("simulated activity recording failure")
}

func TestCreate_ActivityRecordFailureInsideInTx_RollsBackLead(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org}

	txErr := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		r := lead.Repos{Lead: lead.New(tx), Activity: failingRecorder{}}
		created, err := r.Lead.Create(ctx, tenantCtx, minimalInput("Should Roll Back"))
		if err != nil {
			return err
		}
		return r.Activity.Record(ctx, tenantCtx, created.ID, "lead_created", nil, nil)
	})
	if txErr == nil {
		t.Fatal("expected the activity recording failure to abort the transaction")
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE organization_id = $1`, org).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected the lead insert to be rolled back alongside the failed activity, but found %d rows", count)
	}

	// The allocated lead_number must not be burned either — same
	// no-gaps guarantee TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber
	// proves for an INSERT failure; this proves it for an activity
	// failure too, since both share the same db.InTx boundary.
	repo := lead.New(pool)
	next, err := repo.Create(ctx, tenantCtx, minimalInput("Should Get Number 1"))
	if err != nil {
		t.Fatalf("create after rollback: %v", err)
	}
	if next.LeadNumber != 1 {
		t.Errorf("expected lead_number 1 (rolled-back attempt should leave no gap), got %d", next.LeadNumber)
	}
}

// failingEnqueuer satisfies lead.WebhookEnqueuer and always fails —
// standing in for a webhook_deliveries write that goes wrong mid-
// transaction (Phase 7 #101).
type failingEnqueuer struct{}

func (failingEnqueuer) Enqueue(context.Context, tenant.Context, string, []byte) (int, error) {
	return 0, errors.New("simulated webhook enqueue failure")
}

// TestCreate_WebhookEnqueueFailureInsideInTx_RollsBackLead is #101's
// atomicity acceptance criterion against a REAL transaction. The fake
// Store test above it (webhook_trigger_test.go) proves the Go-level
// wiring propagates the error; this proves Postgres actually discards the
// lead insert alongside it.
//
// TD §5's whole argument rests on this: if a lead could commit while its
// deliveries rolled back, the event would be lost with nothing to
// reconstruct it from, which is exactly the failure the in-transaction
// enqueue exists to prevent.
func TestCreate_WebhookEnqueueFailureInsideInTx_RollsBackLead(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org}

	txErr := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		r := lead.Repos{Lead: lead.New(tx), Activity: activity.NewRecorder(tx), Webhook: failingEnqueuer{}}
		created, err := r.Lead.Create(ctx, tenantCtx, minimalInput("Should Roll Back Too"))
		if err != nil {
			return err
		}
		if err := r.Activity.Record(ctx, tenantCtx, created.ID, "lead_created", nil, nil); err != nil {
			return err
		}
		_, err = r.Webhook.Enqueue(ctx, tenantCtx, "lead.created", []byte(`{"event":"lead.created"}`))
		return err
	})
	if txErr == nil {
		t.Fatal("expected the webhook enqueue failure to abort the transaction")
	}

	var leads int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE organization_id = $1`, org).Scan(&leads); err != nil {
		t.Fatalf("query leads: %v", err)
	}
	if leads != 0 {
		t.Errorf("lead survived a failed webhook enqueue: %d rows", leads)
	}

	// The activity written earlier in the same transaction must be gone too.
	var activities int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activities WHERE organization_id = $1`, org).Scan(&activities); err != nil {
		t.Fatalf("query activities: %v", err)
	}
	if activities != 0 {
		t.Errorf("activity survived a failed webhook enqueue: %d rows", activities)
	}
}

// TestCreate_LeadAndDeliveriesCommitTogether is the other half. Rolling
// back on failure means nothing if the success path never wrote the
// deliveries in the first place — this proves one committed transaction
// leaves both the lead and one delivery row per subscribed endpoint.
func TestCreate_LeadAndDeliveriesCommitTogether(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner, PrincipalType: tenant.PrincipalUser}

	endpointID := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `
		INSERT INTO webhook_endpoints (id, organization_id, url, secret_ciphertext, secret_prefix, events)
		VALUES ($1, $2, 'https://receiver.example.com/hook', $3, 'whsec_aaaaaaaa', $4)`,
		endpointID, org, []byte("sealed"), []string{"lead.created"})
	if err != nil {
		t.Fatalf("seed endpoint: %v", err)
	}

	txErr := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		r := lead.Repos{Lead: lead.New(tx), Activity: activity.NewRecorder(tx), Webhook: webhook.NewEnqueuer(tx)}
		created, err := r.Lead.Create(ctx, tenantCtx, minimalInput("Commits Together"))
		if err != nil {
			return err
		}
		if err := r.Activity.Record(ctx, tenantCtx, created.ID, "lead_created", nil, nil); err != nil {
			return err
		}
		_, err = r.Webhook.Enqueue(ctx, tenantCtx, "lead.created", []byte(`{"event":"lead.created"}`))
		return err
	})
	if txErr != nil {
		t.Fatalf("transaction failed: %v", txErr)
	}

	var leads, deliveries int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE organization_id = $1`, org).Scan(&leads); err != nil {
		t.Fatalf("query leads: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE organization_id = $1 AND endpoint_id = $2`, org, endpointID).Scan(&deliveries); err != nil {
		t.Fatalf("query deliveries: %v", err)
	}
	if leads != 1 || deliveries != 1 {
		t.Errorf("expected 1 lead and 1 delivery, got %d and %d", leads, deliveries)
	}
}
