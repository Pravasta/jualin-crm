package task_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/task"
)

func TestRepository_Create_Succeeds(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := task.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil)

	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, task.CreateInput{
		LeadID: leadID, Title: "Follow up",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != task.StatusOpen {
		t.Errorf("expected default status open, got %q", created.Status)
	}
	if created.Version != 1 {
		t.Errorf("expected initial version 1, got %d", created.Version)
	}
}

func TestRepository_Create_InvalidAssignee_ReturnsErrAssigneeNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := task.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil)
	bogus := uuid.Must(uuid.NewV7())

	_, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, task.CreateInput{
		LeadID: leadID, Title: "Follow up", AssignedToMembershipID: &bogus,
	})
	if !errors.Is(err, task.ErrAssigneeNotFound) {
		t.Fatalf("expected task.ErrAssigneeNotFound, got: %v", err)
	}
}

func TestRepository_Create_LeadInAnotherOrg_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := task.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	leadInA := seedLead(t, ctx, pool, orgA, nil)

	_, err := repo.Create(ctx, tenant.Context{OrganizationID: orgB, Role: tenant.RoleOwner}, task.CreateInput{
		LeadID: leadInA, Title: "Follow up",
	})
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

// TestRepository_Employee_VisibilityIsThroughLeadAssignment is the case
// worth a dedicated test: an employee's task visibility is scoped by
// which LEAD they're assigned to, not by the task's own
// assigned_to_membership_id — a task can be assigned to a colleague on
// a lead you own, and you must still see it.
func TestRepository_Employee_VisibilityIsThroughLeadAssignment(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := task.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadOwner := seedMembership(t, ctx, pool, org, "lead-owner@example.com", tenant.RoleEmployee)
	taskAssignee := seedMembership(t, ctx, pool, org, "task-assignee@example.com", tenant.RoleEmployee)
	stranger := seedMembership(t, ctx, pool, org, "stranger@example.com", tenant.RoleEmployee)
	leadID := seedLead(t, ctx, pool, org, &leadOwner)

	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, task.CreateInput{
		LeadID: leadID, Title: "Follow up", AssignedToMembershipID: &taskAssignee,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// The lead's owner must see it, even though the TASK is assigned to
	// someone else.
	if _, err := repo.FindByID(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &leadOwner}, created.ID); err != nil {
		t.Errorf("expected the lead owner to see the task, got: %v", err)
	}

	// The task's own assignee, who does NOT own the lead, must NOT see
	// it — visibility is lead-based, not task-assignee-based.
	if _, err := repo.FindByID(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &taskAssignee}, created.ID); !errors.Is(err, httpx.ErrNotFound) {
		t.Errorf("expected httpx.ErrNotFound for the task's assignee who doesn't own the lead, got: %v", err)
	}

	// An unrelated employee must not see it either.
	if _, err := repo.FindByID(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &stranger}, created.ID); !errors.Is(err, httpx.ErrNotFound) {
		t.Errorf("expected httpx.ErrNotFound for an unrelated employee, got: %v", err)
	}
}

func TestRepository_Complete_SetsStatusCompletedAtCompletedBy(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := task.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil)
	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, task.CreateInput{LeadID: leadID, Title: "Follow up"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	completer := uuid.Must(uuid.NewV7())
	seedSpecificMembership(t, ctx, pool, org, completer, "completer@example.com", tenant.RoleOwner)

	completed, err := repo.Complete(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, created.ID, created.Version, &completer)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.Status != task.StatusDone {
		t.Errorf("expected status done, got %q", completed.Status)
	}
	if completed.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
	if completed.CompletedByMembershipID == nil || *completed.CompletedByMembershipID != completer {
		t.Errorf("expected completed_by %v, got %v", completer, completed.CompletedByMembershipID)
	}
}

func TestRepository_FindAllByLead_InvisibleLead_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := task.New(pool)

	org := seedOrganization(t, ctx, pool)
	other := seedOrganization(t, ctx, pool)
	leadInOther := seedLead(t, ctx, pool, other, nil)

	_, err := repo.FindAllByLead(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, leadInOther)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a lead in a different organization, got: %v", err)
	}
}

func TestRepository_Update_StaleVersion_ReturnsConflictWithCurrentState(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := task.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil)
	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, task.CreateInput{LeadID: leadID, Title: "Original"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newTitle := "Should Not Apply"
	current, err := repo.Update(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, created.ID, created.Version-1, task.UpdateInput{Title: &newTitle})
	if !errors.Is(err, task.ErrVersionConflict) {
		t.Fatalf("expected task.ErrVersionConflict, got: %v", err)
	}
	if current == nil || current.Title == newTitle {
		t.Error("expected current state to be returned and the stale update to not apply")
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
	membershipID := uuid.Must(uuid.NewV7())
	seedSpecificMembership(t, ctx, pool, org, membershipID, email, role)
	return membershipID
}

func seedSpecificMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org, membershipID uuid.UUID, email string, role tenant.Role) {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Test User')`, userID, email); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO memberships (id, organization_id, user_id, role) VALUES ($1, $2, $3, $4)`, membershipID, org, userID, role); err != nil {
		t.Fatalf("failed to seed membership: %v", err)
	}
}

// seedLead inserts a minimal lead row directly via SQL — internal/task
// doesn't depend on internal/lead, so its tests build the lead they
// need with a raw INSERT rather than importing lead.Repository.
func seedLead(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, assignedTo *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	const q = `
		INSERT INTO leads (id, organization_id, lead_number, name, source, assigned_to_membership_id)
		VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), 'Test Lead', 'manual', $3)`
	if _, err := pool.Exec(ctx, q, id, org, assignedTo); err != nil {
		t.Fatalf("failed to seed lead: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE organizations SET next_lead_number = next_lead_number + 1 WHERE id = $1`, org); err != nil {
		t.Fatalf("failed to bump next_lead_number: %v", err)
	}
	return id
}
