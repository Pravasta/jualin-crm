package activity_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/activity/... -run TestUnit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeActivityRepo struct {
	byLead map[uuid.UUID][]*activity.Activity
}

func newFakeActivityRepo() *fakeActivityRepo {
	return &fakeActivityRepo{byLead: map[uuid.UUID][]*activity.Activity{}}
}

func (f *fakeActivityRepo) Create(_ context.Context, t tenant.Context, in activity.CreateInput) (*activity.Activity, error) {
	a := &activity.Activity{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: t.OrganizationID, LeadID: in.LeadID,
		Type: in.Type, ActorMembershipID: in.ActorMembershipID, Body: in.Body,
	}
	f.byLead[in.LeadID] = append(f.byLead[in.LeadID], a)
	return a, nil
}

func (f *fakeActivityRepo) FindAllByLead(_ context.Context, _ tenant.Context, leadID uuid.UUID) ([]*activity.Activity, error) {
	return f.byLead[leadID], nil
}

type fakeStore struct{ repos activity.Repos }

func newFakeStore() *fakeStore {
	return &fakeStore{repos: activity.Repos{Activity: newFakeActivityRepo()}}
}

func (s *fakeStore) InTx(_ context.Context, fn func(activity.Repos) error) error { return fn(s.repos) }
func (s *fakeStore) Repos() activity.Repos                                       { return s.repos }

func actorContext(role tenant.Role) tenant.Context {
	membershipID := uuid.Must(uuid.NewV7())
	return tenant.Context{
		OrganizationID: uuid.Must(uuid.NewV7()), PrincipalType: tenant.PrincipalUser,
		MembershipID: &membershipID, Role: role,
	}
}

// --- tests ---

func TestUnit_Create_EmployeeAllowed(t *testing.T) {
	u := activity.NewUsecase(newFakeStore())
	leadID := uuid.Must(uuid.NewV7())

	_, err := u.Create(context.Background(), actorContext(tenant.RoleEmployee), leadID, activity.CreateActivityInput{Type: "note_added"})
	if err != nil {
		t.Fatalf("expected employee to be allowed to create a user activity, got: %v", err)
	}
}

func TestUnit_Create_UserTypesAccepted(t *testing.T) {
	for _, typ := range []string{"note_added", "call_logged", "whatsapp_opened"} {
		t.Run(typ, func(t *testing.T) {
			u := activity.NewUsecase(newFakeStore())
			leadID := uuid.Must(uuid.NewV7())

			created, err := u.Create(context.Background(), actorContext(tenant.RoleOwner), leadID, activity.CreateActivityInput{Type: typ})
			if err != nil {
				t.Fatalf("expected %s to be accepted, got: %v", typ, err)
			}
			if created.Type != typ {
				t.Errorf("expected type %s, got %s", typ, created.Type)
			}
		})
	}
}

func TestUnit_Create_SystemTypesRejected(t *testing.T) {
	systemTypes := []string{
		"lead_created", "lead_assigned", "lead_unassigned", "status_changed",
		"lead_converted", "task_created", "task_completed",
	}
	for _, typ := range systemTypes {
		t.Run(typ, func(t *testing.T) {
			u := activity.NewUsecase(newFakeStore())
			leadID := uuid.Must(uuid.NewV7())

			_, err := u.Create(context.Background(), actorContext(tenant.RoleOwner), leadID, activity.CreateActivityInput{Type: typ})

			var derr *httpx.DomainError
			if !errors.As(err, &derr) || derr.Status != 422 || derr.Code != "invalid_activity_type" {
				t.Fatalf("expected 422 invalid_activity_type for client-submitted %q, got: %v", typ, err)
			}
		})
	}
}

func TestUnit_Create_ActorIsCaller(t *testing.T) {
	u := activity.NewUsecase(newFakeStore())
	leadID := uuid.Must(uuid.NewV7())
	actor := actorContext(tenant.RoleOwner)

	created, err := u.Create(context.Background(), actor, leadID, activity.CreateActivityInput{Type: "note_added"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ActorMembershipID == nil || *created.ActorMembershipID != *actor.MembershipID {
		t.Errorf("expected actor to be the caller's membership, got %v", created.ActorMembershipID)
	}
}

func TestUnit_List_ReturnsLeadTimeline(t *testing.T) {
	store := newFakeStore()
	u := activity.NewUsecase(store)
	leadID := uuid.Must(uuid.NewV7())
	actor := actorContext(tenant.RoleOwner)

	_, err := u.Create(context.Background(), actor, leadID, activity.CreateActivityInput{Type: "note_added"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := u.List(context.Background(), actor, leadID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(list))
	}
}
