package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/notification"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// TestRepository_CrossRecipientIsolation is TD §8's core requirement:
// "tidak ada endpoint yang bisa membaca notifikasi orang lain, terlepas
// dari role" — proven here even for Owner, which gets no broader access
// to notifications the way it does to leads.
func TestRepository_CrossRecipientIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := notification.New(pool)
	notifier := notification.NewNotifier(pool)

	org := seedOrganization(t, ctx, pool)
	membershipA := seedMembership(t, ctx, pool, org, "a@example.com", tenant.RoleOwner)
	membershipB := seedMembership(t, ctx, pool, org, "b@example.com", tenant.RoleOwner)

	if err := notifier.Notify(ctx, tenant.Context{OrganizationID: org}, membershipA, "lead_assigned", nil, nil, "for A", nil); err != nil {
		t.Fatalf("notify A: %v", err)
	}

	listA, err := repo.FindAllByRecipient(ctx, tenant.Context{OrganizationID: org, MembershipID: &membershipA}, false)
	if err != nil {
		t.Fatalf("find all for A: %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("expected A to see 1 notification, got %d", len(listA))
	}

	listB, err := repo.FindAllByRecipient(ctx, tenant.Context{OrganizationID: org, MembershipID: &membershipB}, false)
	if err != nil {
		t.Fatalf("find all for B: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected B to see 0 notifications (not A's), got %d", len(listB))
	}

	if err := repo.MarkRead(ctx, tenant.Context{OrganizationID: org, MembershipID: &membershipB}, listA[0].ID); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound when B tries to mark A's notification read, got: %v", err)
	}
}

func TestRepository_FindAllByRecipient_UnreadOnly(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := notification.New(pool)
	notifier := notification.NewNotifier(pool)

	org := seedOrganization(t, ctx, pool)
	membershipID := seedMembership(t, ctx, pool, org, "unread@example.com", tenant.RoleOwner)
	tenantCtx := tenant.Context{OrganizationID: org, MembershipID: &membershipID}

	if err := notifier.Notify(ctx, tenant.Context{OrganizationID: org}, membershipID, "lead_assigned", nil, nil, "one", nil); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if err := notifier.Notify(ctx, tenant.Context{OrganizationID: org}, membershipID, "lead_assigned", nil, nil, "two", nil); err != nil {
		t.Fatalf("notify: %v", err)
	}

	all, err := repo.FindAllByRecipient(ctx, tenantCtx, false)
	if err != nil {
		t.Fatalf("find all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(all))
	}

	if err := repo.MarkRead(ctx, tenantCtx, all[0].ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	unread, err := repo.FindAllByRecipient(ctx, tenantCtx, true)
	if err != nil {
		t.Fatalf("find unread: %v", err)
	}
	if len(unread) != 1 {
		t.Fatalf("expected 1 unread notification, got %d", len(unread))
	}
}

func TestRepository_MarkRead_AlreadyRead_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := notification.New(pool)
	notifier := notification.NewNotifier(pool)

	org := seedOrganization(t, ctx, pool)
	membershipID := seedMembership(t, ctx, pool, org, "idempotent@example.com", tenant.RoleOwner)
	tenantCtx := tenant.Context{OrganizationID: org, MembershipID: &membershipID}

	if err := notifier.Notify(ctx, tenant.Context{OrganizationID: org}, membershipID, "lead_assigned", nil, nil, "x", nil); err != nil {
		t.Fatalf("notify: %v", err)
	}
	list, err := repo.FindAllByRecipient(ctx, tenantCtx, false)
	if err != nil || len(list) != 1 {
		t.Fatalf("find all: %v, %d", err, len(list))
	}

	if err := repo.MarkRead(ctx, tenantCtx, list[0].ID); err != nil {
		t.Fatalf("first mark read: %v", err)
	}
	if err := repo.MarkRead(ctx, tenantCtx, list[0].ID); err != nil {
		t.Fatalf("expected re-marking read to succeed, got: %v", err)
	}
}

func TestRepository_MarkAllRead_OnlyAffectsRecipientsOwn(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := notification.New(pool)
	notifier := notification.NewNotifier(pool)

	org := seedOrganization(t, ctx, pool)
	membershipA := seedMembership(t, ctx, pool, org, "markall-a@example.com", tenant.RoleOwner)
	membershipB := seedMembership(t, ctx, pool, org, "markall-b@example.com", tenant.RoleOwner)

	if err := notifier.Notify(ctx, tenant.Context{OrganizationID: org}, membershipA, "lead_assigned", nil, nil, "a", nil); err != nil {
		t.Fatalf("notify a: %v", err)
	}
	if err := notifier.Notify(ctx, tenant.Context{OrganizationID: org}, membershipB, "lead_assigned", nil, nil, "b", nil); err != nil {
		t.Fatalf("notify b: %v", err)
	}

	if err := repo.MarkAllRead(ctx, tenant.Context{OrganizationID: org, MembershipID: &membershipA}); err != nil {
		t.Fatalf("mark all read: %v", err)
	}

	unreadB, err := repo.FindAllByRecipient(ctx, tenant.Context{OrganizationID: org, MembershipID: &membershipB}, true)
	if err != nil {
		t.Fatalf("find unread b: %v", err)
	}
	if len(unreadB) != 1 {
		t.Errorf("expected B's notification to remain unread, got %d unread", len(unreadB))
	}
}

func seedOrganization(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO organizations (id, name) VALUES ($1, 'Test Org')`, id); err != nil {
		t.Fatalf("failed to seed organization: %v", err)
	}
	return id
}

func seedMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, email string, role tenant.Role) uuid.UUID {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Test User')`, userID, email); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	membershipID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, $4)`, membershipID, org, userID, role); err != nil {
		t.Fatalf("failed to seed membership: %v", err)
	}
	return membershipID
}
