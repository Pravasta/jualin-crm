package membership_test

// Integration tests (real Postgres via dbtest) for issue #22's closure
// of the Phase 1 obligation (TD §13): DELETE /v1/memberships/{id} must
// refuse by default while the target still owns open leads, and the
// unassign/reassign escape hatches must be atomic with the deactivation
// and refresh-token revocation #11 already wrote. This is the
// Definition of Done's explicit atomicity requirement, proven against a
// real transaction — not just asserted on a fake.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const testJWTSecret = "membership-handler-test-jwt-secret-32b" // #nosec G101 -- test-only value, not a real credential

type testClaimsParser struct{}

func (testClaimsParser) ParseAccessToken(raw string) (*accesstoken.Claims, error) {
	return accesstoken.Parse([]byte(testJWTSecret), raw)
}

// handlerTestStore wires the REAL auth.NewRefreshTokenRevoker, unlike
// usecase_test.go's package-level testStore (which deliberately no-ops
// it — that file only exercises membership's own SQL). This file's
// whole point is proving refresh-token revocation is atomic with
// deactivation against a real transaction, so it needs the real thing.
type handlerTestStore struct{ pool *pgxpool.Pool }

func newHandlerTestStore(pool *pgxpool.Pool) membership.Store { return &handlerTestStore{pool: pool} }

func (s *handlerTestStore) InTx(ctx context.Context, fn func(membership.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(handlerTestRepos(tx)) })
}

func (s *handlerTestStore) Repos() membership.Repos { return handlerTestRepos(s.pool) }

func handlerTestRepos(q db.Querier) membership.Repos {
	return membership.Repos{
		Member:       membership.New(q),
		Audit:        auditlog.New(q),
		RefreshToken: auth.NewRefreshTokenRevoker(q),
		OpenLead:     lead.NewOpenLeadRepository(q),
		Activity:     activity.NewRecorder(q),
	}
}

func newTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.NewPool(t)
	u := membership.NewUsecase(newHandlerTestStore(pool))

	r := gin.New()
	membership.NewHandler(u).RegisterRoutes(r, authn.Middleware(testClaimsParser{}))
	return r, pool
}

func bearerToken(t *testing.T, userID, orgID, membershipID uuid.UUID, role tenant.Role) string {
	t.Helper()
	tok, err := accesstoken.Issue([]byte(testJWTSecret), 15*time.Minute, userID, orgID, membershipID, role)
	if err != nil {
		t.Fatalf("mint access token: %v", err)
	}
	return tok
}

func doJSON(r *gin.Engine, method, path, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(nil))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// seedLead inserts a minimal lead row directly via SQL — membership
// doesn't depend on internal/lead's domain type, so its tests build the
// lead they need with a raw INSERT rather than importing lead.Repository
// (same pattern activity's and task's tests already use).
func seedLead(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, assignedTo uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	var q string
	var args []any
	if status == "lost" {
		q = `INSERT INTO leads (id, organization_id, lead_number, name, source, assigned_to_membership_id, status, lost_reason)
			VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), 'Test Lead', 'manual', $3, $4, 'other')`
		args = []any{id, org, assignedTo, status}
	} else {
		q = `INSERT INTO leads (id, organization_id, lead_number, name, source, assigned_to_membership_id, status)
			VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), 'Test Lead', 'manual', $3, $4)`
		args = []any{id, org, assignedTo, status}
	}
	if _, err := pool.Exec(ctx, q, args...); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE organizations SET next_lead_number = next_lead_number + 1 WHERE id = $1`, org); err != nil {
		t.Fatalf("bump next_lead_number: %v", err)
	}
	return id
}

// seedRefreshToken inserts a minimal active refresh token for
// membershipID, so tests can confirm it gets revoked.
func seedRefreshToken(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, membershipID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	const q = `
		INSERT INTO refresh_tokens (id, organization_id, membership_id, token_hash, family_id, client, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'dashboard', now() + interval '30 days')`
	if _, err := pool.Exec(ctx, q, id, org, membershipID, id.String(), uuid.Must(uuid.NewV7())); err != nil {
		t.Fatalf("seed refresh token: %v", err)
	}
	return id
}

func TestHandler_Deactivate_OpenLeads_Reject_Returns409AndNothingChanges(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	ownerUser := seedUser(t, ctx, pool, "reject-owner@example.com")
	ownerMembershipID := seedMembership(t, ctx, pool, org, ownerUser, tenant.RoleOwner)
	targetUser := seedUser(t, ctx, pool, "reject-target@example.com")
	targetMembershipID := seedMembership(t, ctx, pool, org, targetUser, tenant.RoleEmployee)
	token := bearerToken(t, ownerUser, org, ownerMembershipID, tenant.RoleOwner)

	seedLead(t, ctx, pool, org, targetMembershipID, "new")
	seedLead(t, ctx, pool, org, targetMembershipID, "contacted")
	seedLead(t, ctx, pool, org, targetMembershipID, "won") // must NOT count as open
	refreshTokenID := seedRefreshToken(t, ctx, pool, org, targetMembershipID)

	w := doJSON(r, http.MethodDelete, "/v1/memberships/"+targetMembershipID.String(), token)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Error struct {
			Code          string `json:"code"`
			OpenLeadCount int    `json:"open_lead_count"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "membership_has_open_leads" || body.Error.OpenLeadCount != 2 {
		t.Fatalf("expected membership_has_open_leads with count 2 (won excluded), got %+v", body)
	}

	// Nothing changed — the whole transaction must have rolled back.
	var deletedAt *string
	if err := pool.QueryRow(ctx, `SELECT deleted_at::text FROM memberships WHERE id = $1`, targetMembershipID).Scan(&deletedAt); err != nil {
		t.Fatalf("query membership: %v", err)
	}
	if deletedAt != nil {
		t.Error("expected the membership to remain active after a rejected deactivation")
	}

	var revokedAt *string
	if err := pool.QueryRow(ctx, `SELECT revoked_at::text FROM refresh_tokens WHERE id = $1`, refreshTokenID).Scan(&revokedAt); err != nil {
		t.Fatalf("query refresh token: %v", err)
	}
	if revokedAt != nil {
		t.Error("expected the refresh token to remain active after a rejected deactivation")
	}
}

func TestHandler_Deactivate_OnOpenLeadsUnassign_AtomicAllOrNothing(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	ownerUser := seedUser(t, ctx, pool, "unassign-owner@example.com")
	ownerMembershipID := seedMembership(t, ctx, pool, org, ownerUser, tenant.RoleOwner)
	targetUser := seedUser(t, ctx, pool, "unassign-target@example.com")
	targetMembershipID := seedMembership(t, ctx, pool, org, targetUser, tenant.RoleEmployee)
	token := bearerToken(t, ownerUser, org, ownerMembershipID, tenant.RoleOwner)

	leadA := seedLead(t, ctx, pool, org, targetMembershipID, "new")
	leadB := seedLead(t, ctx, pool, org, targetMembershipID, "qualified")
	refreshTokenID := seedRefreshToken(t, ctx, pool, org, targetMembershipID)

	w := doJSON(r, http.MethodDelete, "/v1/memberships/"+targetMembershipID.String()+"?on_open_leads=unassign", token)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	for _, leadID := range []uuid.UUID{leadA, leadB} {
		var assignedTo *uuid.UUID
		if err := pool.QueryRow(ctx, `SELECT assigned_to_membership_id FROM leads WHERE id = $1`, leadID).Scan(&assignedTo); err != nil {
			t.Fatalf("query lead %s: %v", leadID, err)
		}
		if assignedTo != nil {
			t.Errorf("expected lead %s to be unassigned, got %v", leadID, assignedTo)
		}
	}

	var activityCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activities WHERE lead_id IN ($1, $2) AND type = 'lead_unassigned'`, leadA, leadB).Scan(&activityCount); err != nil {
		t.Fatalf("query activities: %v", err)
	}
	if activityCount != 2 {
		t.Errorf("expected 2 lead_unassigned activities, got %d", activityCount)
	}

	var deletedAt *string
	if err := pool.QueryRow(ctx, `SELECT deleted_at::text FROM memberships WHERE id = $1`, targetMembershipID).Scan(&deletedAt); err != nil {
		t.Fatalf("query membership: %v", err)
	}
	if deletedAt == nil {
		t.Error("expected the membership to be deactivated")
	}

	// Proves the "semuanya atau tidak sama sekali" atomicity claim
	// against a REAL transaction: the same InTx call that unassigned
	// the leads also deactivated the membership and revoked its
	// refresh token.
	var revokedAt *string
	if err := pool.QueryRow(ctx, `SELECT revoked_at::text FROM refresh_tokens WHERE id = $1`, refreshTokenID).Scan(&revokedAt); err != nil {
		t.Fatalf("query refresh token: %v", err)
	}
	if revokedAt == nil {
		t.Error("expected the refresh token to be revoked atomically with the deactivation")
	}
}

func TestHandler_Deactivate_OnOpenLeadsReassign_MovesLeadsAndLogsActivity(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	ownerUser := seedUser(t, ctx, pool, "reassign-owner@example.com")
	ownerMembershipID := seedMembership(t, ctx, pool, org, ownerUser, tenant.RoleOwner)
	targetUser := seedUser(t, ctx, pool, "reassign-target@example.com")
	targetMembershipID := seedMembership(t, ctx, pool, org, targetUser, tenant.RoleEmployee)
	destUser := seedUser(t, ctx, pool, "reassign-dest@example.com")
	destMembershipID := seedMembership(t, ctx, pool, org, destUser, tenant.RoleEmployee)
	token := bearerToken(t, ownerUser, org, ownerMembershipID, tenant.RoleOwner)

	leadA := seedLead(t, ctx, pool, org, targetMembershipID, "new")

	w := doJSON(r, http.MethodDelete,
		"/v1/memberships/"+targetMembershipID.String()+"?on_open_leads=reassign&reassign_to="+destMembershipID.String(), token)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	var assignedTo uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT assigned_to_membership_id FROM leads WHERE id = $1`, leadA).Scan(&assignedTo); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if assignedTo != destMembershipID {
		t.Errorf("expected lead reassigned to %s, got %s", destMembershipID, assignedTo)
	}

	var activityCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activities WHERE lead_id = $1 AND type = 'lead_assigned'`, leadA).Scan(&activityCount); err != nil {
		t.Fatalf("query activities: %v", err)
	}
	if activityCount != 1 {
		t.Errorf("expected 1 lead_assigned activity, got %d", activityCount)
	}
}

func TestHandler_Deactivate_NoOpenLeads_SucceedsWithDefaultReject(t *testing.T) {
	r, pool := newTestRouter(t)
	ctx := context.Background()
	org := seedOrganization(t, ctx, pool)
	ownerUser := seedUser(t, ctx, pool, "noleads-owner@example.com")
	ownerMembershipID := seedMembership(t, ctx, pool, org, ownerUser, tenant.RoleOwner)
	targetUser := seedUser(t, ctx, pool, "noleads-target@example.com")
	targetMembershipID := seedMembership(t, ctx, pool, org, targetUser, tenant.RoleEmployee)
	token := bearerToken(t, ownerUser, org, ownerMembershipID, tenant.RoleOwner)

	// Only final-status leads — none of these should count as "open".
	seedLead(t, ctx, pool, org, targetMembershipID, "won")
	seedLead(t, ctx, pool, org, targetMembershipID, "lost")
	seedLead(t, ctx, pool, org, targetMembershipID, "unqualified")
	seedLead(t, ctx, pool, org, targetMembershipID, "spam")

	w := doJSON(r, http.MethodDelete, "/v1/memberships/"+targetMembershipID.String(), token)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 (no open leads should block), got %d: %s", w.Code, w.Body.String())
	}
}
