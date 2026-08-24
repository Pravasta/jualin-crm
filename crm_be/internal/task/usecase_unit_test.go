package task_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/task/... -run TestUnit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/task"
)

// --- fakes ---

type fakeTaskRepo struct {
	byID map[uuid.UUID]*task.Task
}

func newFakeTaskRepo() *fakeTaskRepo {
	return &fakeTaskRepo{byID: map[uuid.UUID]*task.Task{}}
}

func (f *fakeTaskRepo) Create(_ context.Context, t tenant.Context, in task.CreateInput) (*task.Task, error) {
	tsk := &task.Task{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: t.OrganizationID, LeadID: in.LeadID,
		Title: in.Title, Description: in.Description, DueAt: in.DueAt, Status: task.StatusOpen,
		AssignedToMembershipID: in.AssignedToMembershipID, Version: 1, CreatedByMembershipID: in.CreatedByMembershipID,
	}
	f.byID[tsk.ID] = tsk
	return tsk, nil
}

func (f *fakeTaskRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*task.Task, error) {
	tsk, ok := f.byID[id]
	if !ok || tsk.OrganizationID != t.OrganizationID || tsk.DeletedAt != nil {
		return nil, httpx.ErrNotFound
	}
	return tsk, nil
}

func (f *fakeTaskRepo) FindAllByOrg(_ context.Context, t tenant.Context, _ task.ListFilter) ([]*task.Task, int, error) {
	var out []*task.Task
	for _, tsk := range f.byID {
		if tsk.OrganizationID == t.OrganizationID && tsk.DeletedAt == nil {
			out = append(out, tsk)
		}
	}
	return out, len(out), nil
}

func (f *fakeTaskRepo) FindAllByLead(_ context.Context, t tenant.Context, leadID uuid.UUID) ([]*task.Task, error) {
	var out []*task.Task
	for _, tsk := range f.byID {
		if tsk.OrganizationID == t.OrganizationID && tsk.LeadID == leadID && tsk.DeletedAt == nil {
			out = append(out, tsk)
		}
	}
	return out, nil
}

func (f *fakeTaskRepo) Update(_ context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in task.UpdateInput) (*task.Task, error) {
	tsk, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if tsk.Version != expectedVersion {
		return tsk, task.ErrVersionConflict
	}
	if in.Title != nil {
		tsk.Title = *in.Title
	}
	if in.AssignedToMembershipID != nil {
		tsk.AssignedToMembershipID = in.AssignedToMembershipID
	}
	tsk.Version++
	return tsk, nil
}

func (f *fakeTaskRepo) Complete(_ context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, completedByMembershipID *uuid.UUID) (*task.Task, error) {
	tsk, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if tsk.Version != expectedVersion {
		return tsk, task.ErrVersionConflict
	}
	tsk.Status = task.StatusDone
	tsk.CompletedByMembershipID = completedByMembershipID
	tsk.Version++
	return tsk, nil
}

func (f *fakeTaskRepo) Delete(_ context.Context, t tenant.Context, id uuid.UUID) error {
	tsk, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return err
	}
	now := tsk.CreatedAt
	tsk.DeletedAt = &now
	return nil
}

// fakeActivityRecorder records every Record call; when err is set, it
// propagates instead of succeeding — used to prove a recording failure
// aborts the whole operation (the wiring half of TD §10's atomicity
// requirement).
type fakeActivityRecorder struct {
	calls []recordedActivity
	err   error
}

type recordedActivity struct {
	leadID       uuid.UUID
	activityType string
	metadata     map[string]any
}

func (f *fakeActivityRecorder) Record(_ context.Context, _ tenant.Context, leadID uuid.UUID, activityType string, _ *uuid.UUID, metadata map[string]any) error {
	f.calls = append(f.calls, recordedActivity{leadID, activityType, metadata})
	return f.err
}

type fakeStore struct {
	repos    task.Repos
	activity *fakeActivityRecorder
}

func newFakeStore() *fakeStore {
	rec := &fakeActivityRecorder{}
	return &fakeStore{repos: task.Repos{Task: newFakeTaskRepo(), Activity: rec}, activity: rec}
}

func (s *fakeStore) InTx(_ context.Context, fn func(task.Repos) error) error { return fn(s.repos) }
func (s *fakeStore) Repos() task.Repos                                       { return s.repos }

func actorContext(orgID, membershipID uuid.UUID, role tenant.Role) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: role}
}

func ownerActor() (tenant.Context, uuid.UUID) {
	org := uuid.Must(uuid.NewV7())
	return actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner), org
}

// --- tests ---

func TestUnit_Create_TitleRequired(t *testing.T) {
	u := task.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	_, err := u.Create(context.Background(), actor, uuid.Must(uuid.NewV7()), task.CreateTaskInput{})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got: %v", err)
	}
}

func TestUnit_Create_RecordsTaskCreatedActivity(t *testing.T) {
	store := newFakeStore()
	u := task.NewUsecase(store)
	actor, _ := ownerActor()
	leadID := uuid.Must(uuid.NewV7())

	created, err := u.Create(context.Background(), actor, leadID, task.CreateTaskInput{Title: "Follow up"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(store.activity.calls) != 1 {
		t.Fatalf("expected exactly one activity recorded, got %d", len(store.activity.calls))
	}
	got := store.activity.calls[0]
	if got.leadID != leadID || got.activityType != "task_created" {
		t.Errorf("expected task_created on lead %v, got %+v", leadID, got)
	}
	if got.metadata["task_id"] != created.ID || got.metadata["title"] != "Follow up" {
		t.Errorf("expected metadata {task_id, title}, got %v", got.metadata)
	}
}

func TestUnit_Create_ActivityRecordFailure_PropagatesError(t *testing.T) {
	store := newFakeStore()
	store.activity.err = errors.New("activity: record failed")
	u := task.NewUsecase(store)
	actor, _ := ownerActor()

	_, err := u.Create(context.Background(), actor, uuid.Must(uuid.NewV7()), task.CreateTaskInput{Title: "Follow up"})
	if err == nil {
		t.Fatal("expected Create to fail when recording the activity fails")
	}
}

func TestUnit_Update_StaleVersion_ReturnsVersionConflict(t *testing.T) {
	store := newFakeStore()
	u := task.NewUsecase(store)
	actor, _ := ownerActor()
	created, err := u.Create(context.Background(), actor, uuid.Must(uuid.NewV7()), task.CreateTaskInput{Title: "Follow up"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newTitle := "Updated"
	_, err = u.Update(context.Background(), actor, created.ID, task.UpdateTaskInput{Version: created.Version - 1, Title: &newTitle})

	var conflict *task.VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *task.VersionConflictError, got: %v", err)
	}
}

func TestUnit_Update_NoActivityRecorded(t *testing.T) {
	store := newFakeStore()
	u := task.NewUsecase(store)
	actor, _ := ownerActor()
	created, _ := u.Create(context.Background(), actor, uuid.Must(uuid.NewV7()), task.CreateTaskInput{Title: "Follow up"})
	store.activity.calls = nil // drop the task_created call from Create

	newTitle := "Updated"
	_, err := u.Update(context.Background(), actor, created.ID, task.UpdateTaskInput{Version: created.Version, Title: &newTitle})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(store.activity.calls) != 0 {
		t.Errorf("expected no activity for a general update (no task_updated type exists), got %d", len(store.activity.calls))
	}
}

func TestUnit_Complete_SetsStatusAndRecordsTaskCompleted(t *testing.T) {
	store := newFakeStore()
	u := task.NewUsecase(store)
	actor, _ := ownerActor()
	leadID := uuid.Must(uuid.NewV7())
	created, _ := u.Create(context.Background(), actor, leadID, task.CreateTaskInput{Title: "Follow up"})
	store.activity.calls = nil

	completed, err := u.Complete(context.Background(), actor, created.ID, created.Version)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.Status != task.StatusDone {
		t.Errorf("expected status done, got %q", completed.Status)
	}
	if completed.CompletedByMembershipID == nil || *completed.CompletedByMembershipID != *actor.MembershipID {
		t.Errorf("expected completed_by to be the caller, got %v", completed.CompletedByMembershipID)
	}

	if len(store.activity.calls) != 1 || store.activity.calls[0].activityType != "task_completed" {
		t.Fatalf("expected exactly one task_completed activity, got %+v", store.activity.calls)
	}
	if store.activity.calls[0].leadID != leadID {
		t.Errorf("expected activity on lead %v, got %v", leadID, store.activity.calls[0].leadID)
	}
}

func TestUnit_Complete_StaleVersion_ReturnsVersionConflict(t *testing.T) {
	store := newFakeStore()
	u := task.NewUsecase(store)
	actor, _ := ownerActor()
	created, _ := u.Create(context.Background(), actor, uuid.Must(uuid.NewV7()), task.CreateTaskInput{Title: "Follow up"})

	_, err := u.Complete(context.Background(), actor, created.ID, created.Version-1)

	var conflict *task.VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *task.VersionConflictError, got: %v", err)
	}
}

func TestUnit_Delete_EmployeeForbidden(t *testing.T) {
	store := newFakeStore()
	u := task.NewUsecase(store)
	actor, org := ownerActor()
	created, _ := u.Create(context.Background(), actor, uuid.Must(uuid.NewV7()), task.CreateTaskInput{Title: "Follow up"})

	employeeCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	err := u.Delete(context.Background(), employeeCtx, created.ID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for employee delete, got: %v", err)
	}
}

func TestUnit_Delete_ManagerAllowed(t *testing.T) {
	store := newFakeStore()
	u := task.NewUsecase(store)
	actor, org := ownerActor()
	created, _ := u.Create(context.Background(), actor, uuid.Must(uuid.NewV7()), task.CreateTaskInput{Title: "Follow up"})

	managerCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleManager)
	if err := u.Delete(context.Background(), managerCtx, created.ID); err != nil {
		t.Fatalf("expected manager to be allowed to delete, got: %v", err)
	}
}
