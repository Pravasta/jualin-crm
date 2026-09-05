package notification_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/notification/... -run TestUnit

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/notification"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeNotificationRepo struct {
	byID map[uuid.UUID]*notification.Notification
}

func newFakeNotificationRepo() *fakeNotificationRepo {
	return &fakeNotificationRepo{byID: map[uuid.UUID]*notification.Notification{}}
}

func (f *fakeNotificationRepo) FindAllByRecipient(_ context.Context, t tenant.Context, unreadOnly bool) ([]*notification.Notification, error) {
	var out []*notification.Notification
	for _, n := range f.byID {
		if n.OrganizationID != t.OrganizationID || (t.MembershipID == nil || n.RecipientMembershipID != *t.MembershipID) {
			continue
		}
		if unreadOnly && n.ReadAt != nil {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (f *fakeNotificationRepo) MarkRead(_ context.Context, t tenant.Context, id uuid.UUID) error {
	n, ok := f.byID[id]
	if !ok || n.OrganizationID != t.OrganizationID || t.MembershipID == nil || n.RecipientMembershipID != *t.MembershipID {
		return httpx.ErrNotFound
	}
	if n.ReadAt == nil {
		now := n.CreatedAt
		n.ReadAt = &now
	}
	return nil
}

func (f *fakeNotificationRepo) MarkAllRead(_ context.Context, t tenant.Context) error {
	for _, n := range f.byID {
		if n.OrganizationID == t.OrganizationID && t.MembershipID != nil && n.RecipientMembershipID == *t.MembershipID && n.ReadAt == nil {
			now := n.CreatedAt
			n.ReadAt = &now
		}
	}
	return nil
}

// ExistsThisMonth exists so *fakeNotificationRepo still satisfies
// notification.Repository (#123). Nothing in this file exercises the
// quota-notification threshold — that lives in cmd/api's composition
// root — so always-false is both honest and enough.
func (f *fakeNotificationRepo) ExistsThisMonth(_ context.Context, _ tenant.Context, _ string) (bool, error) {
	return false, nil
}

type fakeStore struct{ repo *fakeNotificationRepo }

func newFakeStore() *fakeStore {
	return &fakeStore{repo: newFakeNotificationRepo()}
}

func (s *fakeStore) InTx(_ context.Context, fn func(notification.Repos) error) error {
	return fn(notification.Repos{Notification: s.repo})
}
func (s *fakeStore) Repos() notification.Repos {
	return notification.Repos{Notification: s.repo}
}

func actorContext(orgID, membershipID uuid.UUID) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: tenant.RoleOwner}
}

// --- tests ---

func TestUnit_List_OnlyOwnNotifications(t *testing.T) {
	store := newFakeStore()
	u := notification.NewUsecase(store)
	org := uuid.Must(uuid.NewV7())
	me, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	store.repo.byID[uuid.Must(uuid.NewV7())] = &notification.Notification{ID: uuid.Must(uuid.NewV7()), OrganizationID: org, RecipientMembershipID: me, Type: "lead_assigned", Title: "mine"}
	store.repo.byID[uuid.Must(uuid.NewV7())] = &notification.Notification{ID: uuid.Must(uuid.NewV7()), OrganizationID: org, RecipientMembershipID: other, Type: "lead_assigned", Title: "not mine"}

	list, err := u.List(context.Background(), actorContext(org, me), false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Title != "mine" {
		t.Fatalf("expected only the recipient's own notification, got %+v", list)
	}
}

func TestUnit_List_UnreadOnlyFilter(t *testing.T) {
	store := newFakeStore()
	u := notification.NewUsecase(store)
	org, me := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	unread := &notification.Notification{ID: uuid.Must(uuid.NewV7()), OrganizationID: org, RecipientMembershipID: me, Type: "lead_assigned", Title: "unread"}
	store.repo.byID[unread.ID] = unread

	read := &notification.Notification{ID: uuid.Must(uuid.NewV7()), OrganizationID: org, RecipientMembershipID: me, Type: "lead_assigned", Title: "read"}
	store.repo.byID[read.ID] = read
	if err := u.MarkRead(context.Background(), actorContext(org, me), read.ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	list, err := u.List(context.Background(), actorContext(org, me), true)
	if err != nil {
		t.Fatalf("list unread: %v", err)
	}
	if len(list) != 1 || list[0].Title != "unread" {
		t.Fatalf("expected only the unread notification, got %+v", list)
	}
}

func TestUnit_MarkRead_AlreadyRead_IsIdempotent(t *testing.T) {
	store := newFakeStore()
	u := notification.NewUsecase(store)
	org, me := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	n := &notification.Notification{ID: uuid.Must(uuid.NewV7()), OrganizationID: org, RecipientMembershipID: me, Type: "lead_assigned", Title: "x"}
	store.repo.byID[n.ID] = n

	if err := u.MarkRead(context.Background(), actorContext(org, me), n.ID); err != nil {
		t.Fatalf("first mark read: %v", err)
	}
	if err := u.MarkRead(context.Background(), actorContext(org, me), n.ID); err != nil {
		t.Fatalf("expected marking an already-read notification read again to succeed, got: %v", err)
	}
}

func TestUnit_MarkRead_NotOwnNotification_ReturnsNotFound(t *testing.T) {
	store := newFakeStore()
	u := notification.NewUsecase(store)
	org, owner, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	n := &notification.Notification{ID: uuid.Must(uuid.NewV7()), OrganizationID: org, RecipientMembershipID: owner, Type: "lead_assigned", Title: "x"}
	store.repo.byID[n.ID] = n

	err := u.MarkRead(context.Background(), actorContext(org, other), n.ID)
	if err != httpx.ErrNotFound {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestUnit_MarkAllRead_MarksOnlyOwnUnread(t *testing.T) {
	store := newFakeStore()
	u := notification.NewUsecase(store)
	org, me, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	mine := &notification.Notification{ID: uuid.Must(uuid.NewV7()), OrganizationID: org, RecipientMembershipID: me, Type: "lead_assigned", Title: "mine"}
	store.repo.byID[mine.ID] = mine
	theirs := &notification.Notification{ID: uuid.Must(uuid.NewV7()), OrganizationID: org, RecipientMembershipID: other, Type: "lead_assigned", Title: "theirs"}
	store.repo.byID[theirs.ID] = theirs

	if err := u.MarkAllRead(context.Background(), actorContext(org, me)); err != nil {
		t.Fatalf("mark all read: %v", err)
	}
	if mine.ReadAt == nil {
		t.Error("expected my notification to be marked read")
	}
	if theirs.ReadAt != nil {
		t.Error("expected another recipient's notification to remain unread")
	}
}
