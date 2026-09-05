package lead_test

// TestUnit_* tests prove Usecase is decoupled from PostgreSQL (ADR-011) —
// fake Store, no Docker. Run in isolation with:
//
//	go test ./internal/lead/... -run TestUnit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// --- fakes ---

type fakeLeadRepo struct {
	byID       map[uuid.UUID]*lead.Lead
	nextNumber int

	cleanupCalls []uuid.UUID // orgs CleanupExpiredIdempotencyKeys was called for
}

func newFakeLeadRepo() *fakeLeadRepo {
	return &fakeLeadRepo{byID: map[uuid.UUID]*lead.Lead{}, nextNumber: 1}
}

func (f *fakeLeadRepo) CleanupExpiredIdempotencyKeys(_ context.Context, t tenant.Context) error {
	f.cleanupCalls = append(f.cleanupCalls, t.OrganizationID)
	return nil
}

// CountCreatedThisMonth exists so *fakeLeadRepo still satisfies
// lead.Repository (#122). Nothing in this file exercises the quota —
// the meter has no reader inside lead.Usecase until #123 wires
// enforcement — so counting what the fake happens to hold is both
// honest and enough.
func (f *fakeLeadRepo) CountCreatedThisMonth(_ context.Context, t tenant.Context) (int, error) {
	n := 0
	for _, l := range f.byID {
		if l.OrganizationID == t.OrganizationID {
			n++
		}
	}
	return n, nil
}

func (f *fakeLeadRepo) Create(_ context.Context, t tenant.Context, in lead.CreateInput) (*lead.Lead, error) {
	if in.IdempotencyKey != nil {
		for _, l := range f.byID {
			if l.IdempotencyKey != nil && *l.IdempotencyKey == *in.IdempotencyKey && l.OrganizationID == t.OrganizationID {
				return nil, lead.ErrIdempotencyKeyExists
			}
		}
	}
	l := &lead.Lead{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: t.OrganizationID, LeadNumber: f.nextNumber,
		Name: in.Name, Email: in.Email, Phone: in.Phone, PhoneE164: in.PhoneE164, Company: in.Company, Notes: in.Notes,
		Status: "new", Source: in.Source, AssignedToMembershipID: in.AssignedToMembershipID,
		IdempotencyKey: in.IdempotencyKey, Version: 1, CreatedByMembershipID: in.CreatedByMembershipID,
		RawPayload: in.RawPayload, SourceAPIKeyID: in.SourceAPIKeyID, SourceFormID: in.SourceFormID,
	}
	f.nextNumber++
	f.byID[l.ID] = l
	return l, nil
}

func (f *fakeLeadRepo) FindByID(_ context.Context, t tenant.Context, id uuid.UUID) (*lead.Lead, error) {
	l, ok := f.byID[id]
	if !ok || l.OrganizationID != t.OrganizationID || l.DeletedAt != nil {
		return nil, httpx.ErrNotFound
	}
	if t.Role == tenant.RoleEmployee && (l.AssignedToMembershipID == nil || t.MembershipID == nil || *l.AssignedToMembershipID != *t.MembershipID) {
		return nil, httpx.ErrNotFound
	}
	return l, nil
}

func (f *fakeLeadRepo) FindByIdempotencyKey(_ context.Context, t tenant.Context, key string) (*lead.Lead, error) {
	for _, l := range f.byID {
		if l.OrganizationID == t.OrganizationID && l.IdempotencyKey != nil && *l.IdempotencyKey == key {
			return l, nil
		}
	}
	return nil, httpx.ErrNotFound
}

func (f *fakeLeadRepo) FindAllByOrg(_ context.Context, t tenant.Context, _ lead.ListFilter) ([]*lead.Lead, int, error) {
	var out []*lead.Lead
	for _, l := range f.byID {
		if l.OrganizationID == t.OrganizationID && l.DeletedAt == nil {
			out = append(out, l)
		}
	}
	return out, len(out), nil
}

func (f *fakeLeadRepo) Update(_ context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in lead.UpdateInput) (*lead.Lead, error) {
	l, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if l.Version != expectedVersion {
		return l, lead.ErrVersionConflict
	}
	if in.Name != nil {
		l.Name = *in.Name
	}
	if in.Email != nil {
		l.Email = in.Email
	}
	l.Version++
	return l, nil
}

func (f *fakeLeadRepo) UpdateStatus(_ context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, status string, lostReason *string) (*lead.Lead, error) {
	l, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if l.Version != expectedVersion {
		return l, lead.ErrVersionConflict
	}
	l.Status = status
	l.LostReason = lostReason
	l.Version++
	return l, nil
}

func (f *fakeLeadRepo) UpdateAssignment(_ context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, assignedTo *uuid.UUID) (*lead.Lead, error) {
	l, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return nil, err
	}
	if l.Version != expectedVersion {
		return l, lead.ErrVersionConflict
	}
	l.AssignedToMembershipID = assignedTo
	l.Version++
	return l, nil
}

func (f *fakeLeadRepo) Delete(_ context.Context, t tenant.Context, id uuid.UUID) error {
	l, err := f.FindByID(context.Background(), t, id)
	if err != nil {
		return err
	}
	now := l.CreatedAt
	l.DeletedAt = &now
	return nil
}

// recordedActivity captures one fakeActivityRecorder.Record call.
type recordedActivity struct {
	leadID            uuid.UUID
	activityType      string
	actorMembershipID *uuid.UUID
	metadata          map[string]any
}

// fakeActivityRecorder lets tests assert Create/UpdateStatus call
// Record with the right arguments, and that a Record failure
// propagates as an error from the whole InTx call (the wiring half of
// the atomicity acceptance criterion — the other half, that a real
// Postgres transaction actually rolls back, is proven separately in
// repository_test.go against a live database).
type fakeActivityRecorder struct {
	calls []recordedActivity
	err   error
}

func (f *fakeActivityRecorder) Record(_ context.Context, _ tenant.Context, leadID uuid.UUID, activityType string, actorMembershipID *uuid.UUID, metadata map[string]any) error {
	f.calls = append(f.calls, recordedActivity{leadID, activityType, actorMembershipID, metadata})
	return f.err
}

// recordedNotification captures one fakeNotificationSender.Notify call.
type recordedNotification struct {
	recipientMembershipID uuid.UUID
	notifType             string
	leadID                *uuid.UUID
	taskID                *uuid.UUID
	title                 string
	body                  *string
}

// fakeNotificationSender lets tests assert UpdateAssignment calls
// Notify (or doesn't, for self-assignment) with the right arguments.
type fakeNotificationSender struct {
	calls []recordedNotification
}

func (f *fakeNotificationSender) Notify(_ context.Context, _ tenant.Context, recipientMembershipID uuid.UUID, notifType string, leadID, taskID *uuid.UUID, title string, body *string) error {
	f.calls = append(f.calls, recordedNotification{recipientMembershipID, notifType, leadID, taskID, title, body})
	return nil
}

// fakeWebhookEnqueuer records what lead handed to the outbound-webhook
// queue. It deliberately keeps the raw payload bytes rather than a parsed
// struct — the assertions are about the exact JSON a receiver would get.
type fakeWebhookEnqueuer struct {
	calls []enqueuedEvent
	err   error
}

type enqueuedEvent struct {
	event   string
	payload []byte
}

func (f *fakeWebhookEnqueuer) Enqueue(_ context.Context, _ tenant.Context, eventType string, payload []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.calls = append(f.calls, enqueuedEvent{event: eventType, payload: payload})
	return len(f.calls), nil
}

type fakeStore struct {
	repo         *fakeLeadRepo
	repos        lead.Repos
	activity     *fakeActivityRecorder
	notification *fakeNotificationSender
	webhook      *fakeWebhookEnqueuer
}

func newFakeStore() *fakeStore {
	repo := newFakeLeadRepo()
	rec := &fakeActivityRecorder{}
	notif := &fakeNotificationSender{}
	hook := &fakeWebhookEnqueuer{}
	return &fakeStore{
		repo:         repo,
		repos:        lead.Repos{Lead: repo, Activity: rec, Notification: notif, Webhook: hook},
		activity:     rec,
		notification: notif,
		webhook:      hook,
	}
}

// newFakeStoreWithFailingActivity is newFakeStore, but Activity.Record
// always fails — used to prove a recording failure aborts the whole
// operation instead of silently succeeding with no activity written.
func newFakeStoreWithFailingActivity() *fakeStore {
	s := newFakeStore()
	s.activity.err = errors.New("activity: record failed")
	return s
}

func (s *fakeStore) InTx(_ context.Context, fn func(lead.Repos) error) error { return fn(s.repos) }
func (s *fakeStore) Repos() lead.Repos                                       { return s.repos }

func actorContext(orgID, membershipID uuid.UUID, role tenant.Role) tenant.Context {
	return tenant.Context{OrganizationID: orgID, PrincipalType: tenant.PrincipalUser, MembershipID: &membershipID, Role: role}
}

func ownerActor() (tenant.Context, uuid.UUID) {
	org := uuid.Must(uuid.NewV7())
	return actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleOwner), org
}

// --- tests ---

func TestUnit_Create_EmployeeForbidden(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	org := uuid.Must(uuid.NewV7())

	_, _, err := u.Create(context.Background(), actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee), lead.CreateLeadInput{Name: "Budi"})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden, got: %v", err)
	}
}

func TestUnit_Create_NameRequired(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	_, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got: %v", err)
	}
}

func TestUnit_Create_DefaultsSourceToManual(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	created, isNew, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true for a fresh create")
	}
	if created.Source != "manual" {
		t.Errorf("expected default source 'manual', got %q", created.Source)
	}
}

func TestUnit_Create_InvalidSource_Rejected(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	_, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", Source: "carrier-pigeon"})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for an invalid source, got: %v", err)
	}
}

func TestUnit_Create_NormalizesEmailAndPhone(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	email := "Budi@Example.COM"
	phoneRaw := "0812-3456-7890"
	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", Email: &email, Phone: &phoneRaw})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Email == nil || *created.Email != "budi@example.com" {
		t.Errorf("expected lowercased email, got %v", created.Email)
	}
	if created.PhoneE164 == nil || *created.PhoneE164 != "+6281234567890" {
		t.Errorf("expected normalized phone_e164, got %v", created.PhoneE164)
	}
	if created.Phone == nil || *created.Phone != phoneRaw {
		t.Errorf("expected raw phone preserved, got %v", created.Phone)
	}
}

func TestUnit_Create_UnparseablePhone_StillAccepted(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	badPhone := "1234"
	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", Phone: &badPhone})
	if err != nil {
		t.Fatalf("expected an unparseable phone to still be accepted, got: %v", err)
	}
	if created.PhoneE164 != nil {
		t.Errorf("expected nil phone_e164 for an unparseable number, got %v", *created.PhoneE164)
	}
	if created.Phone == nil || *created.Phone != badPhone {
		t.Error("expected the raw phone value to be preserved even when unparseable")
	}
}

func TestUnit_Create_IdempotentReplay_ReturnsSameLeadNotNew(t *testing.T) {
	u := lead.NewUsecase(newFakeStore())
	actor, _ := ownerActor()

	key := "client-generated-key"
	first, isNew1, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", IdempotencyKey: &key})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if !isNew1 {
		t.Fatal("expected the first create to be new")
	}

	second, isNew2, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Different Name Ignored", IdempotencyKey: &key})
	if err != nil {
		t.Fatalf("replay create: %v", err)
	}
	if isNew2 {
		t.Error("expected the replay to report isNew=false")
	}
	if second.ID != first.ID {
		t.Errorf("expected the replay to return the SAME lead, got a different id")
	}
}

func TestUnit_Get_Employee_OtherPersonsLead_ReturnsNotFound(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, org := ownerActor()

	otherEmployee := uuid.Must(uuid.NewV7())
	assignedIn := lead.CreateLeadInput{Name: "Assigned Elsewhere", AssignedToMembershipID: &otherEmployee}
	created, _, err := u.Create(context.Background(), actor, assignedIn)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	me := uuid.Must(uuid.NewV7())
	_, err = u.Get(context.Background(), actorContext(org, me, tenant.RoleEmployee), created.ID)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestUnit_Update_StaleVersion_ReturnsVersionConflict(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "Updated"
	_, err = u.Update(context.Background(), actor, created.ID, lead.UpdateLeadInput{Version: created.Version - 1, Name: &newName})

	var conflict *lead.VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *lead.VersionConflictError, got: %v", err)
	}
	if conflict.Current == nil || conflict.Current.ID != created.ID {
		t.Error("expected the conflict to carry the current lead")
	}
}

func TestUnit_Delete_ManagerForbidden(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, org := ownerActor()

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	managerCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleManager)
	err = u.Delete(context.Background(), managerCtx, created.ID)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for manager delete, got: %v", err)
	}
}

// --- status transition tests (TD §5) ---

func TestUnit_UpdateStatus_NewToWon_Rejected(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	_, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "won"})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_status_transition" {
		t.Fatalf("expected invalid_status_transition, got: %v", err)
	}
}

func TestUnit_UpdateStatus_NewToContacted_Accepted(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	updated, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "contacted"})
	if err != nil {
		t.Fatalf("expected new->contacted to succeed, got: %v", err)
	}
	if updated.Status != "contacted" {
		t.Errorf("expected status contacted, got %q", updated.Status)
	}
}

func TestUnit_UpdateStatus_QualifiedToContacted_Accepted(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	step1, _ := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "contacted"})
	step2, _ := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: step1.Version, Status: "qualified"})

	updated, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: step2.Version, Status: "contacted"})
	if err != nil {
		t.Fatalf("expected qualified->contacted (one step back, B7) to succeed, got: %v", err)
	}
	if updated.Status != "contacted" {
		t.Errorf("expected status contacted, got %q", updated.Status)
	}
}

func TestUnit_UpdateStatus_Lost_RequiresReason(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	_, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "lost"})

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for lost without a reason, got: %v", err)
	}
}

func TestUnit_UpdateStatus_LostWithReason_Accepted(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	reason := "price"
	updated, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "lost", LostReason: &reason})
	if err != nil {
		t.Fatalf("expected lost with a reason to succeed, got: %v", err)
	}
	if updated.LostReason == nil || *updated.LostReason != "price" {
		t.Errorf("expected lost_reason 'price', got %v", updated.LostReason)
	}
}

func TestUnit_UpdateStatus_LeavingLost_ClearsReason(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	reason := "timing"
	lost, _ := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "lost", LostReason: &reason})

	// Approximation documented in usecase.go: leaving lost allows any
	// main-path status, not specifically the one it left from.
	revived, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: lost.Version, Status: "qualified"})
	if err != nil {
		t.Fatalf("expected leaving lost to succeed, got: %v", err)
	}
	if revived.LostReason != nil {
		t.Errorf("expected lost_reason to be cleared, got %v", *revived.LostReason)
	}
}

func TestUnit_UpdateStatus_UnqualifiedIsFinal(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	unqualified, _ := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "unqualified"})

	_, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: unqualified.Version, Status: "new"})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "invalid_status_transition" {
		t.Fatalf("expected unqualified to be final (invalid_status_transition), got: %v", err)
	}
}

// --- auto-log tests (TD §10) ---

func TestUnit_Create_RecordsLeadCreatedActivity(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(store.activity.calls) != 1 {
		t.Fatalf("expected exactly one activity recorded, got %d", len(store.activity.calls))
	}
	got := store.activity.calls[0]
	if got.leadID != created.ID {
		t.Errorf("expected activity leadID %v, got %v", created.ID, got.leadID)
	}
	if got.activityType != "lead_created" {
		t.Errorf("expected activity type lead_created, got %q", got.activityType)
	}
}

func TestUnit_Create_ActivityRecordFailure_PropagatesError(t *testing.T) {
	store := newFakeStoreWithFailingActivity()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()

	_, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	if err == nil {
		t.Fatal("expected Create to fail when recording the activity fails")
	}
}

func TestUnit_UpdateStatus_RecordsStatusChangedActivityWithFromTo(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	store.activity.calls = nil // drop the lead_created call from Create

	_, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "contacted"})
	if err != nil {
		t.Fatalf("update status: %v", err)
	}

	if len(store.activity.calls) != 1 {
		t.Fatalf("expected exactly one activity recorded, got %d", len(store.activity.calls))
	}
	got := store.activity.calls[0]
	if got.activityType != "status_changed" {
		t.Errorf("expected activity type status_changed, got %q", got.activityType)
	}
	if got.metadata["from"] != "new" || got.metadata["to"] != "contacted" {
		t.Errorf("expected metadata {from: new, to: contacted}, got %v", got.metadata)
	}
}

func TestUnit_UpdateStatus_ActivityRecordFailure_PropagatesError(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	store.activity.err = errors.New("activity: record failed")
	_, err := u.UpdateStatus(context.Background(), actor, created.ID, lead.UpdateStatusInput{Version: created.Version, Status: "contacted"})
	if err == nil {
		t.Fatal("expected UpdateStatus to fail when recording the activity fails")
	}
}

// --- assignment tests (TD §11) ---

func TestUnit_UpdateAssignment_EmployeeForbidden(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	org, ownerMembershipID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	owner := actorContext(org, ownerMembershipID, tenant.RoleOwner)
	created, _, _ := u.Create(context.Background(), owner, lead.CreateLeadInput{Name: "Budi"})

	employeeCtx := actorContext(org, uuid.Must(uuid.NewV7()), tenant.RoleEmployee)
	someone := uuid.Must(uuid.NewV7())
	_, err := u.UpdateAssignment(context.Background(), employeeCtx, created.ID, lead.UpdateAssignmentInput{Version: created.Version, AssignedToMembershipID: &someone})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "forbidden" {
		t.Fatalf("expected forbidden for employee assignment, got: %v", err)
	}
}

func TestUnit_UpdateAssignment_ToOther_RecordsActivityAndNotifies(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})
	store.activity.calls = nil // drop lead_created

	assignee := uuid.Must(uuid.NewV7())
	updated, err := u.UpdateAssignment(context.Background(), actor, created.ID, lead.UpdateAssignmentInput{Version: created.Version, AssignedToMembershipID: &assignee})
	if err != nil {
		t.Fatalf("update assignment: %v", err)
	}
	if updated.AssignedToMembershipID == nil || *updated.AssignedToMembershipID != assignee {
		t.Errorf("expected assignee %s, got %v", assignee, updated.AssignedToMembershipID)
	}

	if len(store.activity.calls) != 1 || store.activity.calls[0].activityType != "lead_assigned" {
		t.Fatalf("expected 1 lead_assigned activity, got %+v", store.activity.calls)
	}
	if store.activity.calls[0].metadata["to"] != assignee {
		t.Errorf("expected metadata.to = %s, got %v", assignee, store.activity.calls[0].metadata)
	}

	if len(store.notification.calls) != 1 {
		t.Fatalf("expected exactly 1 notification, got %d", len(store.notification.calls))
	}
	if store.notification.calls[0].recipientMembershipID != assignee {
		t.Errorf("expected notification recipient %s, got %s", assignee, store.notification.calls[0].recipientMembershipID)
	}
}

// TestUnit_UpdateAssignment_ToSelf_NoNotification is TD §11's explicit
// rule: "memberi tahu seseorang tentang tindakannya sendiri hanya
// menambah bising" — the activity still fires, but Notify must not be
// called.
func TestUnit_UpdateAssignment_ToSelf_NoNotification(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	org, ownerMembershipID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	owner := actorContext(org, ownerMembershipID, tenant.RoleOwner)
	created, _, _ := u.Create(context.Background(), owner, lead.CreateLeadInput{Name: "Budi"})
	store.activity.calls = nil

	_, err := u.UpdateAssignment(context.Background(), owner, created.ID, lead.UpdateAssignmentInput{Version: created.Version, AssignedToMembershipID: &ownerMembershipID})
	if err != nil {
		t.Fatalf("update assignment: %v", err)
	}

	if len(store.activity.calls) != 1 || store.activity.calls[0].activityType != "lead_assigned" {
		t.Fatalf("expected the lead_assigned activity to still fire, got %+v", store.activity.calls)
	}
	if len(store.notification.calls) != 0 {
		t.Errorf("expected no notification for self-assignment, got %d", len(store.notification.calls))
	}
}

func TestUnit_UpdateAssignment_Unassign_RecordsActivityNoNotification(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	assignee := uuid.Must(uuid.NewV7())
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", AssignedToMembershipID: &assignee})
	store.activity.calls = nil

	updated, err := u.UpdateAssignment(context.Background(), actor, created.ID, lead.UpdateAssignmentInput{Version: created.Version, AssignedToMembershipID: nil})
	if err != nil {
		t.Fatalf("update assignment: %v", err)
	}
	if updated.AssignedToMembershipID != nil {
		t.Error("expected assignment to be cleared")
	}

	if len(store.activity.calls) != 1 || store.activity.calls[0].activityType != "lead_unassigned" {
		t.Fatalf("expected 1 lead_unassigned activity, got %+v", store.activity.calls)
	}
	fromPtr, ok := store.activity.calls[0].metadata["from"].(*uuid.UUID)
	if !ok || fromPtr == nil || *fromPtr != assignee {
		t.Errorf("expected metadata.from = %s, got %v", assignee, store.activity.calls[0].metadata)
	}
	if len(store.notification.calls) != 0 {
		t.Errorf("expected no notification for unassignment, got %d", len(store.notification.calls))
	}
}

func TestUnit_UpdateAssignment_StaleVersion_ReturnsVersionConflict(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()
	created, _, _ := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"})

	assignee := uuid.Must(uuid.NewV7())
	_, err := u.UpdateAssignment(context.Background(), actor, created.ID, lead.UpdateAssignmentInput{Version: created.Version - 1, AssignedToMembershipID: &assignee})

	var conflict *lead.VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *lead.VersionConflictError, got: %v", err)
	}
}

// --- Create: public API principal (Phase 4 #47) ---

func apiKeyActor(org, apiKeyID uuid.UUID, scopes []string) tenant.Context {
	return tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalAPIKey, APIKeyID: &apiKeyID, Scopes: scopes}
}

func TestUnit_Create_APIKey_AssignedToMembershipID_Rejected(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor := apiKeyActor(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), []string{"leads:write"})
	assignee := uuid.Must(uuid.NewV7())

	_, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", AssignedToMembershipID: &assignee})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "insufficient_scope" {
		t.Fatalf("expected insufficient_scope, got: %v", err)
	}
	if len(store.repo.byID) != 0 {
		t.Error("expected no lead to have been created")
	}
}

// TestUnit_Create_APIKey_SourceAlwaysForcedToAPI proves the body's own
// "source" value (even a legitimate one like "manual") is overridden,
// not merely defaulted — TD §5: "body diabaikan bila mengirim source".
func TestUnit_Create_APIKey_SourceAlwaysForcedToAPI(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	apiKeyID := uuid.Must(uuid.NewV7())
	actor := apiKeyActor(uuid.Must(uuid.NewV7()), apiKeyID, []string{"leads:write"})

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", Source: "manual"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Source != "api" {
		t.Errorf("expected source forced to %q, got %q", "api", created.Source)
	}
	if created.SourceAPIKeyID == nil || *created.SourceAPIKeyID != apiKeyID {
		t.Errorf("expected source_api_key_id %s, got %v", apiKeyID, created.SourceAPIKeyID)
	}
	if created.CreatedByMembershipID != nil {
		t.Errorf("expected created_by_membership_id nil for an api_key create, got %v", created.CreatedByMembershipID)
	}
}

func TestUnit_Create_APIKey_RawPayloadStored(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor := apiKeyActor(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), []string{"leads:write"})
	raw := []byte(`{"name":"Budi","utm_source":"facebook"}`)

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", RawPayload: raw})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if string(created.RawPayload) != string(raw) {
		t.Errorf("expected raw_payload stored verbatim, got %q", created.RawPayload)
	}
}

// TestUnit_Create_APIKey_CleanupThrottledPerOrganization proves the
// idempotency-key sweep (TD §7) runs at most once per organization per
// throttle window — two creates for the SAME org in quick succession
// must produce exactly one cleanup call.
func TestUnit_Create_APIKey_CleanupThrottledPerOrganization(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	org := uuid.Must(uuid.NewV7())
	actor := apiKeyActor(org, uuid.Must(uuid.NewV7()), []string{"leads:write"})

	if _, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Ani"}); err != nil {
		t.Fatalf("second create: %v", err)
	}

	if len(store.repo.cleanupCalls) != 1 {
		t.Fatalf("expected exactly 1 cleanup call across 2 creates within the throttle window, got %d", len(store.repo.cleanupCalls))
	}
	if store.repo.cleanupCalls[0] != org {
		t.Errorf("expected cleanup called for org %s, got %s", org, store.repo.cleanupCalls[0])
	}
}

// TestUnit_Create_UserPrincipal_NeverTriggersCleanup proves the sweep
// is scoped to the API key path only — a dashboard create (principal
// user) must never call it, matching TD §7's "Pada POST /v1/leads jalur
// API key" scoping.
func TestUnit_Create_UserPrincipal_NeverTriggersCleanup(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor, _ := ownerActor()

	if _, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(store.repo.cleanupCalls) != 0 {
		t.Errorf("expected no cleanup call for a user-principal create, got %d", len(store.repo.cleanupCalls))
	}
}

// --- Create: public form principal (Phase 6 #87) ---

func formActor(org, formID uuid.UUID) tenant.Context {
	return tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalPublicForm, FormID: &formID}
}

func TestUnit_Create_PublicForm_AssignedToMembershipID_Rejected(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor := formActor(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
	assignee := uuid.Must(uuid.NewV7())

	_, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", AssignedToMembershipID: &assignee})

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Code != "insufficient_scope" {
		t.Fatalf("expected insufficient_scope, got: %v", err)
	}
	if len(store.repo.byID) != 0 {
		t.Error("expected no lead to have been created")
	}
}

// TestUnit_Create_PublicForm_SourceAlwaysForcedToForm mirrors
// TestUnit_Create_APIKey_SourceAlwaysForcedToAPI — the body's own
// "source" is overridden, not merely defaulted, and source_form_id is
// stamped from the resolved principal, never from the request.
func TestUnit_Create_PublicForm_SourceAlwaysForcedToForm(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	formID := uuid.Must(uuid.NewV7())
	actor := formActor(uuid.Must(uuid.NewV7()), formID)

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", Source: "manual"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Source != "form" {
		t.Errorf("expected source forced to %q, got %q", "form", created.Source)
	}
	if created.SourceFormID == nil || *created.SourceFormID != formID {
		t.Errorf("expected source_form_id %s, got %v", formID, created.SourceFormID)
	}
	if created.SourceAPIKeyID != nil {
		t.Errorf("expected source_api_key_id nil for a form create, got %v", created.SourceAPIKeyID)
	}
	if created.CreatedByMembershipID != nil {
		t.Errorf("expected created_by_membership_id nil for a form create, got %v", created.CreatedByMembershipID)
	}
}

func TestUnit_Create_PublicForm_RawPayloadStored(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor := formActor(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
	raw := []byte(`{"name":"Budi","product":"Paket A"}`)

	created, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi", RawPayload: raw})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if string(created.RawPayload) != string(raw) {
		t.Errorf("expected raw_payload stored verbatim, got %q", created.RawPayload)
	}
}

// TestUnit_Create_PublicForm_NeverTriggersCleanup is
// TestUnit_Create_UserPrincipal_NeverTriggersCleanup's form-principal
// counterpart — TD §5 is explicit forms never send an Idempotency-Key,
// so the sweep (scoped to the API key path only) must never fire here
// either.
func TestUnit_Create_PublicForm_NeverTriggersCleanup(t *testing.T) {
	store := newFakeStore()
	u := lead.NewUsecase(store)
	actor := formActor(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

	if _, _, err := u.Create(context.Background(), actor, lead.CreateLeadInput{Name: "Budi"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(store.repo.cleanupCalls) != 0 {
		t.Errorf("expected no cleanup call for a public-form create, got %d", len(store.repo.cleanupCalls))
	}
}
