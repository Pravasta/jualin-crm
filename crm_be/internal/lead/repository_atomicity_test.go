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

	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
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
