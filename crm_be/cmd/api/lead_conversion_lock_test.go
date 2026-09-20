package main

// Issue #142 / ADR-016: once a lead is converted to a Customer its status is
// locked. The unit tests in internal/lead prove the rule with a fake; what
// only a real database can prove is that a status change and a conversion
// arriving TOGETHER can never both win.
//
// The bug this guards is a race, and a race test that merely fires two
// requests and hopes is the trap issue #19 already documented: on a fast
// machine it can stay green forever while the logic is wrong. So the core
// tests here are DETERMINISTIC — one side is held open inside a transaction
// while the other is proven to be blocked behind it — and the stress test is
// only a supplement. All of it goes through the production router.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/customer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type lockFixture struct {
	pool         *pgxpool.Pool
	router       *gin.Engine
	org          uuid.UUID
	membershipID uuid.UUID
	token        string
}

func newLockFixture(t *testing.T) lockFixture {
	t.Helper()
	r, pool := newIsolationRouter(t)
	org, user, membershipID := seedOrgOwner(t, pool, "Lock Org", "lock-owner@example.com")
	return lockFixture{
		pool:         pool,
		router:       r,
		org:          org,
		membershipID: membershipID,
		token:        mintBearerToken(t, user, org, membershipID, tenant.RoleOwner),
	}
}

func (f lockFixture) tenantCtx() tenant.Context {
	return tenant.Context{OrganizationID: f.org, MembershipID: &f.membershipID, Role: tenant.RoleOwner, PrincipalType: tenant.PrincipalUser}
}

func (f lockFixture) do(method, path string, body any) *httptest.ResponseRecorder {
	reader := bytes.NewReader(nil)
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.token)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func (f lockFixture) patchStatus(leadID uuid.UUID, version int, status string) *httptest.ResponseRecorder {
	return f.do(http.MethodPatch, "/v1/leads/"+leadID.String()+"/status", map[string]any{"version": version, "status": status})
}

func (f lockFixture) convert(leadID uuid.UUID) *httptest.ResponseRecorder {
	return f.do(http.MethodPost, "/v1/leads/"+leadID.String()+"/convert", map[string]any{})
}

func (f lockFixture) leadStatus(t *testing.T, leadID uuid.UUID) string {
	t.Helper()
	var s string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM leads WHERE id = $1`, leadID).Scan(&s); err != nil {
		t.Fatalf("read lead status: %v", err)
	}
	return s
}

func (f lockFixture) customerCount(t *testing.T, leadID uuid.UUID) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM customers WHERE converted_from_lead_id = $1`, leadID).Scan(&n); err != nil {
		t.Fatalf("count customers: %v", err)
	}
	return n
}

func lockErrCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v (%s)", err, w.Body.String())
	}
	return env.Error.Code
}

// waitForLockWaiter blocks until some backend is waiting on a LOCK — the
// proof that the request we just started is genuinely parked behind the
// transaction we are holding open, rather than still on its way to the
// database or already finished. Without it "the request did not return yet"
// would be indistinguishable from "the request has not started yet".
func waitForLockWaiter(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&n); err != nil {
			t.Fatalf("read pg_stat_activity: %v", err)
		}
		if n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no backend ever waited on a lock — the request was not blocked behind the open transaction, so the two sides do not exclude each other")
}

func requireStillPending(t *testing.T, done <-chan *httptest.ResponseRecorder, what string) {
	t.Helper()
	select {
	case w := <-done:
		t.Fatalf("%s returned (%d) while the other transaction was still open — it must wait: %s", what, w.Code, w.Body.String())
	case <-time.After(150 * time.Millisecond):
	}
}

// Order 1: a conversion is IN FLIGHT (customer inserted, lead row locked,
// not yet committed) when the status change arrives. The change must wait,
// then be refused — it may not read "not converted yet" and slip through.
//
// The conversion is the REAL customer repository's SQL run inside our own
// open transaction, not a hand-written stand-in for it.
func TestConversionLock_StatusChangeWaitsForInFlightConversion_ThenIsRefused(t *testing.T) {
	f := newLockFixture(t)
	leadID := seedIsolationLead(t, f.pool, f.org, "won")

	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background()) //nolint:errcheck // no-op after Commit

	if _, err := customer.New(tx).Convert(t.Context(), f.tenantCtx(), leadID, &f.membershipID); err != nil {
		t.Fatalf("in-flight convert: %v", err)
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.patchStatus(leadID, 1, "proposal") }()

	waitForLockWaiter(t, f.pool)
	requireStillPending(t, done, "PATCH status")

	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("commit conversion: %v", err)
	}

	w := <-done
	if w.Code != http.StatusUnprocessableEntity || lockErrCode(t, w) != "lead_converted_locked" {
		t.Fatalf("expected 422 lead_converted_locked once the conversion committed, got %d: %s", w.Code, w.Body.String())
	}
	if got := f.leadStatus(t, leadID); got != "won" {
		t.Errorf("the refused change must leave the lead won, got %q", got)
	}
	if n := f.customerCount(t, leadID); n != 1 {
		t.Errorf("expected exactly 1 customer, got %d", n)
	}
}

// Order 2: a status change is IN FLIGHT (lead moved off `won`, not yet
// committed) when the conversion arrives. Convert must wait, then find the
// lead is no longer won and refuse — it may not read the stale `won` and
// create a customer for a lead that is now "Penawaran".
func TestConversionLock_ConvertWaitsForInFlightStatusChange_ThenIsRefused(t *testing.T) {
	f := newLockFixture(t)
	leadID := seedIsolationLead(t, f.pool, f.org, "won")

	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background()) //nolint:errcheck // no-op after Commit

	leads := lead.New(tx)
	cur, err := leads.FindByIDForUpdate(t.Context(), f.tenantCtx(), leadID)
	if err != nil {
		t.Fatalf("lock lead: %v", err)
	}
	if _, err := leads.UpdateStatus(t.Context(), f.tenantCtx(), leadID, cur.Version, "proposal", nil); err != nil {
		t.Fatalf("in-flight status change: %v", err)
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- f.convert(leadID) }()

	waitForLockWaiter(t, f.pool)
	requireStillPending(t, done, "POST convert")

	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("commit status change: %v", err)
	}

	w := <-done
	if w.Code != http.StatusUnprocessableEntity || lockErrCode(t, w) != "invalid_status_transition" {
		t.Fatalf("expected 422 invalid_status_transition (lead no longer won), got %d: %s", w.Code, w.Body.String())
	}
	if n := f.customerCount(t, leadID); n != 0 {
		t.Errorf("no customer may exist for a lead that left `won`, got %d", n)
	}
	if got := f.leadStatus(t, leadID); got != "proposal" {
		t.Errorf("expected the committed status change to stand, got %q", got)
	}
}

// The plain end-to-end path through the real router, no race: after a
// conversion, every destination is refused and the lead stays won — and
// nothing ELSE about the lead is locked (ADR-016: only status).
func TestConversionLock_AfterConversion_StatusRefusedButEditingStillWorks(t *testing.T) {
	f := newLockFixture(t)
	leadID := seedIsolationLead(t, f.pool, f.org, "won")

	if w := f.convert(leadID); w.Code != http.StatusCreated {
		t.Fatalf("convert: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	for _, to := range []string{"proposal", "lost", "unqualified", "spam"} {
		body := map[string]any{"version": 1, "status": to}
		if to == "lost" {
			body["lost_reason"] = "price"
		}
		w := f.do(http.MethodPatch, "/v1/leads/"+leadID.String()+"/status", body)
		if w.Code != http.StatusUnprocessableEntity || lockErrCode(t, w) != "lead_converted_locked" {
			t.Errorf("-> %s: expected 422 lead_converted_locked, got %d: %s", to, w.Code, w.Body.String())
		}
	}
	if got := f.leadStatus(t, leadID); got != "won" {
		t.Errorf("status must stay won, got %q", got)
	}

	w := f.do(http.MethodPatch, "/v1/leads/"+leadID.String(), map[string]any{"version": 1, "name": "Nama Dikoreksi"})
	if w.Code != http.StatusOK {
		t.Fatalf("editing a converted lead's name must still work, got %d: %s", w.Code, w.Body.String())
	}
}

// The supplement: many status-change/conversion pairs actually released
// together. Each pair must have exactly ONE winner, and no customer may ever
// exist for a lead that is not `won`. On its own this could stay green on a
// fast machine while the logic is wrong (issue #19), which is why the two
// deterministic tests above carry the proof.
func TestConversionLock_ConcurrentPairs_NeverLeaveACustomerForANonWonLead(t *testing.T) {
	f := newLockFixture(t)
	const pairs = 40

	for i := 0; i < pairs; i++ {
		leadID := seedIsolationLead(t, f.pool, f.org, "won")

		start := make(chan struct{})
		var wg sync.WaitGroup
		var patch, conv *httptest.ResponseRecorder
		wg.Add(2)
		go func() { defer wg.Done(); <-start; patch = f.patchStatus(leadID, 1, "proposal") }()
		go func() { defer wg.Done(); <-start; conv = f.convert(leadID) }()
		close(start)
		wg.Wait()

		patchOK := patch.Code == http.StatusOK
		convOK := conv.Code == http.StatusCreated
		if patchOK == convOK {
			t.Fatalf("pair %d: exactly one side may win, got patch=%d convert=%d (patch: %s | convert: %s)",
				i, patch.Code, conv.Code, patch.Body.String(), conv.Body.String())
		}
	}

	var bad int
	if err := f.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM customers c
		JOIN leads l ON l.id = c.converted_from_lead_id AND l.organization_id = c.organization_id
		WHERE l.status <> 'won'`).Scan(&bad); err != nil {
		t.Fatalf("invariant query: %v", err)
	}
	if bad != 0 {
		t.Fatalf("%d customer(s) exist for a lead that is no longer won — the two operations interleaved", bad)
	}
}

// IsConverted's tenant scoping and its soft-delete rule, against real
// Postgres: another organization can never see a conversion, and deleting
// the customer must NOT reopen the lead (uq_customers_org_lead is not a
// partial index, so the lead can never be converted again either).
func TestConversionLock_IsConverted_TenantScopedAndSurvivesCustomerSoftDelete(t *testing.T) {
	f := newLockFixture(t)
	orgB, _, _ := seedOrgOwner(t, f.pool, "Other Org", "lock-other@example.com")
	leadID := seedIsolationLead(t, f.pool, f.org, "won")
	repo := lead.New(f.pool)

	if got, _ := repo.IsConverted(t.Context(), f.tenantCtx(), leadID); got {
		t.Fatal("a lead with no customer must not read as converted")
	}

	customerID := seedIsolationCustomer(t, f.pool, f.org, leadID)

	if got, err := repo.IsConverted(t.Context(), f.tenantCtx(), leadID); err != nil || !got {
		t.Fatalf("own organization must see the conversion, got %v / %v", got, err)
	}
	if got, err := repo.IsConverted(t.Context(), tenant.Context{OrganizationID: orgB}, leadID); err != nil || got {
		t.Fatalf("ANOTHER organization must never see this conversion, got %v / %v", got, err)
	}

	if _, err := f.pool.Exec(t.Context(), `UPDATE customers SET deleted_at = now() WHERE id = $1`, customerID); err != nil {
		t.Fatalf("soft-delete customer: %v", err)
	}
	if got, err := repo.IsConverted(t.Context(), f.tenantCtx(), leadID); err != nil || !got {
		t.Fatalf("deleting the customer must not unlock the lead, got %v / %v", got, err)
	}
}
