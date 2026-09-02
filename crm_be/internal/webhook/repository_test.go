package webhook_test

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

func tctx(org uuid.UUID) tenant.Context {
	return tenant.Context{OrganizationID: org, Role: tenant.RoleOwner, PrincipalType: tenant.PrincipalUser}
}

func newTestEndpoint() *webhook.Endpoint {
	return &webhook.Endpoint{
		ID:               uuid.Must(uuid.NewV7()),
		URL:              "https://example.com/hook",
		SecretCiphertext: []byte("sealed-secret-not-a-real-ciphertext"),
		SecretPrefix:     "whsec_" + strings.Repeat("b", 8), // #nosec G101 -- test fixture, not a real credential
		Events:           []string{webhook.EventLeadCreated},
		Description:      "",
		IsActive:         true,
	}
}

func TestRepository_Create_FindByID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)

	org := seedOrg(t, ctx, pool)
	tc := tctx(org)
	e := newTestEndpoint()
	e.Events = []string{webhook.EventLeadCreated, webhook.EventLeadStatusChanged}

	if err := repo.Create(ctx, tc, e); err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be populated")
	}

	found, err := repo.FindByID(ctx, tc, e.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.URL != e.URL || !bytes.Equal(found.SecretCiphertext, e.SecretCiphertext) || !found.IsActive {
		t.Errorf("round-trip mismatch: %+v", found)
	}
	if len(found.Events) != 2 || found.Events[0] != webhook.EventLeadCreated || found.Events[1] != webhook.EventLeadStatusChanged {
		t.Errorf("events did not round-trip in order: %v", found.Events)
	}
}

func TestRepository_FindByID_CrossOrg_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)

	orgA := seedOrg(t, ctx, pool)
	orgB := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tctx(orgA), e); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := repo.FindByID(ctx, tctx(orgB), e.ID); err != httpx.ErrNotFound {
		t.Fatalf("expected httpx.ErrNotFound for cross-org, got %v", err)
	}
}

func TestRepository_Update_PartialLeavesUntouched(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)

	org := seedOrg(t, ctx, pool)
	tc := tctx(org)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tc, e); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Change only the events; URL and is_active must be left alone.
	newEvents := []string{webhook.EventLeadStatusChanged}
	updated, err := repo.Update(ctx, tc, e.ID, webhook.UpdateInput{Events: &newEvents})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.URL != e.URL {
		t.Errorf("URL changed unexpectedly: %q", updated.URL)
	}
	if !updated.IsActive {
		t.Error("is_active flipped unexpectedly")
	}
	if len(updated.Events) != 1 || updated.Events[0] != webhook.EventLeadStatusChanged {
		t.Errorf("events not updated: %v", updated.Events)
	}

	// Deactivate via is_active only.
	inactive := false
	updated2, err := repo.Update(ctx, tc, e.ID, webhook.UpdateInput{IsActive: &inactive})
	if err != nil {
		t.Fatalf("update deactivate: %v", err)
	}
	if updated2.IsActive {
		t.Error("expected is_active=false after deactivate")
	}
	if len(updated2.Events) != 1 {
		t.Errorf("events changed during deactivate: %v", updated2.Events)
	}
}

func TestRepository_Delete_SoftDelete_ThenNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)

	org := seedOrg(t, ctx, pool)
	tc := tctx(org)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tc, e); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(ctx, tc, e.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.FindByID(ctx, tc, e.ID); err != httpx.ErrNotFound {
		t.Fatalf("expected ErrNotFound after soft delete, got %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_endpoints WHERE id = $1 AND deleted_at IS NOT NULL`, e.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected the row to still exist with deleted_at set, got count=%d", count)
	}
}

func TestRepository_Create_EmptyEvents_RejectedAtDBLevel(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)

	org := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	e.Events = []string{}
	err := repo.Create(ctx, tctx(org), e)
	if err == nil {
		t.Fatal("expected ck_webhook_endpoints_events_not_empty to reject an empty events array")
	}
	if !strings.Contains(err.Error(), "ck_webhook_endpoints_events_not_empty") {
		t.Errorf("expected the events CHECK to fire, got: %v", err)
	}
}

func TestRepository_Create_BadScheme_RejectedAtDBLevel(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)

	org := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	e.URL = "ftp://example.com/hook"
	err := repo.Create(ctx, tctx(org), e)
	if err == nil || !strings.Contains(err.Error(), "ck_webhook_endpoints_url_scheme") {
		t.Errorf("expected the url_scheme CHECK to fire, got: %v", err)
	}
}

func TestRepository_Create_CompositeFKOnCreatedBy(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)

	orgA := seedOrg(t, ctx, pool)
	orgB := seedOrg(t, ctx, pool)
	// A membership that exists, but in the WRONG organization.
	memB := seedMembership(t, ctx, pool, orgB, "webhook-fk-b@example.com")

	e := newTestEndpoint()
	e.CreatedByMembershipID = &memB
	err := repo.Create(ctx, tctx(orgA), e)
	if err == nil || !strings.Contains(err.Error(), "fk_webhook_endpoints_created_by") {
		t.Errorf("expected the composite FK to reject a membership from another org, got: %v", err)
	}
}

// --- webhook_deliveries ---

func TestDeliveryRepository_Enqueue_FindByEndpoint_Pagination(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	org := seedOrg(t, ctx, pool)
	tc := tctx(org)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tc, e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	for i := 0; i < 5; i++ {
		d := &webhook.Delivery{
			ID:         uuid.Must(uuid.NewV7()),
			EndpointID: e.ID,
			EventType:  webhook.EventLeadCreated,
			Payload:    []byte(`{"n":` + strconv.Itoa(i) + `}`),
		}
		if err := drepo.Enqueue(ctx, tc, d); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		if d.Status != webhook.StatusPending {
			t.Errorf("expected new delivery to be pending, got %s", d.Status)
		}
	}

	page1, total, err := drepo.FindByEndpoint(ctx, tc, e.ID, 1, 3)
	if err != nil {
		t.Fatalf("find page 1: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(page1) != 3 {
		t.Errorf("page 1 len = %d, want 3", len(page1))
	}

	page2, _, err := drepo.FindByEndpoint(ctx, tc, e.ID, 2, 3)
	if err != nil {
		t.Fatalf("find page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page 2 len = %d, want 2", len(page2))
	}
}

func TestDeliveryRepository_FindByID_CrossOrg_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	orgA := seedOrg(t, ctx, pool)
	orgB := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tctx(orgA), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	d := &webhook.Delivery{ID: uuid.Must(uuid.NewV7()), EndpointID: e.ID, EventType: webhook.EventLeadCreated, Payload: []byte(`{}`)}
	if err := drepo.Enqueue(ctx, tctx(orgA), d); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := drepo.FindByID(ctx, tctx(orgB), d.ID); err != httpx.ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-org delivery, got %v", err)
	}
}

func TestDeliveryRepository_MarkForRetry(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	org := seedOrg(t, ctx, pool)
	tc := tctx(org)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tc, e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	failedID := seedDelivery(t, ctx, pool, org, e.ID, "failed", time.Now())
	pendingID := seedDelivery(t, ctx, pool, org, e.ID, "pending", time.Now())

	d, err := drepo.MarkForRetry(ctx, tc, failedID)
	if err != nil {
		t.Fatalf("mark failed for retry: %v", err)
	}
	if d.Status != webhook.StatusPending || d.Attempt != 0 || d.Error != nil {
		t.Errorf("failed delivery not reset cleanly: %+v", d)
	}

	if _, err := drepo.MarkForRetry(ctx, tc, pendingID); err != webhook.ErrDeliveryNotRetryable {
		t.Fatalf("expected ErrDeliveryNotRetryable for a pending delivery, got %v", err)
	}
}

func TestDeliveryRepository_ClaimDue(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	org := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tctx(org), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	dueNow := seedDelivery(t, ctx, pool, org, e.ID, "pending", time.Now().Add(-time.Minute))
	_ = seedDelivery(t, ctx, pool, org, e.ID, "pending", time.Now().Add(time.Hour)) // not due yet
	_ = seedDelivery(t, ctx, pool, org, e.ID, "succeeded", time.Now().Add(-time.Hour))

	claimed, err := drepo.ClaimDue(ctx, 10)
	if err != nil {
		t.Fatalf("claim due: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != dueNow {
		t.Fatalf("expected exactly the one due pending row, got %d rows", len(claimed))
	}
	if claimed[0].Status != webhook.StatusDelivering || claimed[0].DeliveringSince == nil {
		t.Errorf("claimed row not marked delivering: %+v", claimed[0])
	}

	// A second claim finds nothing — the row is already delivering.
	again, err := drepo.ClaimDue(ctx, 10)
	if err != nil {
		t.Fatalf("claim again: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("expected no rows on the second claim, got %d", len(again))
	}
}

func TestDeliveryRepository_ClaimDue_IsIndexHit(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SET enable_seqscan = off"); err != nil {
		t.Fatalf("disable seqscan: %v", err)
	}

	rows, err := conn.Query(ctx, `EXPLAIN SELECT id FROM webhook_deliveries WHERE status = 'pending' AND next_attempt_at <= now() ORDER BY next_attempt_at LIMIT 20`)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan: %v", err)
		}
		plan.WriteString(line + "\n")
	}
	if !strings.Contains(plan.String(), "ix_webhook_deliveries_claim") {
		t.Fatalf("expected the claim query to use ix_webhook_deliveries_claim, got:\n%s", plan.String())
	}
}

func TestDeliveryRepository_Reap(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	org := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tctx(org), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	stuck := seedDeliveryDelivering(t, ctx, pool, org, e.ID, time.Now().Add(-15*time.Minute))
	fresh := seedDeliveryDelivering(t, ctx, pool, org, e.ID, time.Now().Add(-1*time.Minute))

	n, err := drepo.Reap(ctx, time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if n != 1 {
		t.Errorf("reaped %d, want 1", n)
	}

	assertStatus(t, ctx, pool, stuck, "pending")
	assertStatus(t, ctx, pool, fresh, "delivering")
}

func TestDeliveryRepository_Purge_NeverTouchesPending(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := webhook.New(pool)
	drepo := webhook.NewDeliveryRepository(pool)

	org := seedOrg(t, ctx, pool)
	e := newTestEndpoint()
	if err := repo.Create(ctx, tctx(org), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	old := time.Now().Add(-40 * 24 * time.Hour)
	oldSucceeded := seedDeliveryAged(t, ctx, pool, org, e.ID, "succeeded", old)
	oldFailed := seedDeliveryAged(t, ctx, pool, org, e.ID, "failed", old)
	oldPending := seedDeliveryAged(t, ctx, pool, org, e.ID, "pending", old)

	n, err := drepo.Purge(ctx, time.Now().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 2 {
		t.Errorf("purged %d, want 2 (succeeded + failed)", n)
	}

	assertGone(t, ctx, pool, oldSucceeded)
	assertGone(t, ctx, pool, oldFailed)
	assertExists(t, ctx, pool, oldPending)
}

// --- seed helpers ---

func seedOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Webhook Test Org')`, id); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return id
}

func seedMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, email string) uuid.UUID {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Test User')`, userID, email); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	memID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, 'owner')`, memID, org, userID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return memID
}

func seedDelivery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, endpoint uuid.UUID, status string, nextAttempt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, organization_id, endpoint_id, event_type, payload, status, next_attempt_at, error)
		VALUES ($1, $2, $3, 'lead.created', '{}'::jsonb, $4, $5, 'previous failure')`,
		id, org, endpoint, status, nextAttempt)
	if err != nil {
		t.Fatalf("seed delivery: %v", err)
	}
	return id
}

func seedDeliveryDelivering(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, endpoint uuid.UUID, since time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, organization_id, endpoint_id, event_type, payload, status, delivering_since)
		VALUES ($1, $2, $3, 'lead.created', '{}'::jsonb, 'delivering', $4)`,
		id, org, endpoint, since)
	if err != nil {
		t.Fatalf("seed delivering delivery: %v", err)
	}
	return id
}

func seedDeliveryAged(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, endpoint uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (id, organization_id, endpoint_id, event_type, payload, status, created_at)
		VALUES ($1, $2, $3, 'lead.created', '{}'::jsonb, $4, $5)`,
		id, org, endpoint, status, createdAt)
	if err != nil {
		t.Fatalf("seed aged delivery: %v", err)
	}
	return id
}

func assertStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(ctx, `SELECT status FROM webhook_deliveries WHERE id = $1`, id).Scan(&got); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if got != want {
		t.Errorf("delivery %s: status = %q, want %q", id, got, want)
	}
}

func assertGone(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("expected delivery %s to be purged", id)
	}
}

func assertExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected delivery %s to survive purge", id)
	}
}
