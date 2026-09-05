package invitation_test

// Postgres-backed repository tests — the seat-counting predicate
// (Phase 8.5 #124) is exactly the kind of thing a fake can assert
// without ever proving the real SQL matches (excludes expired/
// accepted/revoked, isolates tenants). This file closes a gap left by
// #122, which added CountPendingSeats without a repository test of its
// own — #124 is the first real caller depending on its correctness.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/invitation"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func seedRealInvitation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, invitedBy uuid.UUID, email string, expiresAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx,
		`INSERT INTO invitations (id, organization_id, email, role, token_hash, invited_by_membership_id, expires_at)
		 VALUES ($1, $2, $3, 'employee', $4, $5, $6)`,
		id, org, email, "hash-"+id.String(), invitedBy, expiresAt,
	); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	return id
}

func TestRepository_CountPendingSeats_CountsUntouchedInvitation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := invitation.New(pool)

	org := seedOrg(t, ctx, pool, "Seat Count Org")
	inviter := seedOwnerMembership(t, ctx, pool, org, "inviter@example.com")
	seedRealInvitation(t, ctx, pool, org, inviter, "pending@example.com", time.Now().Add(48*time.Hour))

	n, err := repo.CountPendingSeats(ctx, tenant.Context{OrganizationID: org})
	if err != nil {
		t.Fatalf("count pending seats: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 pending seat, got %d", n)
	}
}

func TestRepository_CountPendingSeats_ExcludesAcceptedRevokedAndExpired(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := invitation.New(pool)

	org := seedOrg(t, ctx, pool, "Seat Exclude Org")
	inviter := seedOwnerMembership(t, ctx, pool, org, "inviter2@example.com")

	pending := seedRealInvitation(t, ctx, pool, org, inviter, "still-pending@example.com", time.Now().Add(48*time.Hour))
	accepted := seedRealInvitation(t, ctx, pool, org, inviter, "accepted@example.com", time.Now().Add(48*time.Hour))
	revoked := seedRealInvitation(t, ctx, pool, org, inviter, "revoked@example.com", time.Now().Add(48*time.Hour))
	expired := seedRealInvitation(t, ctx, pool, org, inviter, "expired@example.com", time.Now().Add(-1*time.Hour))

	if _, err := pool.Exec(ctx, `UPDATE invitations SET accepted_at = now() WHERE id = $1`, accepted); err != nil {
		t.Fatalf("mark accepted: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE invitations SET revoked_at = now() WHERE id = $1`, revoked); err != nil {
		t.Fatalf("mark revoked: %v", err)
	}
	_ = expired // already expires_at in the past — no update needed

	n, err := repo.CountPendingSeats(ctx, tenant.Context{OrganizationID: org})
	if err != nil {
		t.Fatalf("count pending seats: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 (only %q still pending), got %d", pending, n)
	}
}

func TestRepository_CountPendingSeats_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := invitation.New(pool)

	orgA := seedOrg(t, ctx, pool, "Seat Isolation Org A")
	orgB := seedOrg(t, ctx, pool, "Seat Isolation Org B")
	inviterB := seedOwnerMembership(t, ctx, pool, orgB, "inviter-b@example.com")
	seedRealInvitation(t, ctx, pool, orgB, inviterB, "pending-b@example.com", time.Now().Add(48*time.Hour))

	n, err := repo.CountPendingSeats(ctx, tenant.Context{OrganizationID: orgA})
	if err != nil {
		t.Fatalf("count pending seats: %v", err)
	}
	if n != 0 {
		t.Errorf("expected org A to count 0 of org B's pending invitations, got %d", n)
	}
}
