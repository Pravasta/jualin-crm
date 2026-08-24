package activity_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// TestRepository_Create_MetadataRoundTrips is the first jsonb WRITE this
// codebase exercises against a real database (lead.RawPayload is
// written-through but never actually populated by any caller) — the
// project's own discipline is to verify claims like "pgx accepts a
// []byte json.Marshal result for a jsonb column" directly rather than
// assume it, per notes.md's #19/#20 entries.
func TestRepository_Create_MetadataRoundTrips(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := activity.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil)

	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, activity.CreateInput{
		LeadID: leadID, Type: "status_changed",
		Metadata: map[string]any{"from": "new", "to": "contacted"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(created.Metadata, &decoded); err != nil {
		t.Fatalf("expected metadata to be valid JSON, got %q: %v", created.Metadata, err)
	}
	if decoded["from"] != "new" || decoded["to"] != "contacted" {
		t.Errorf("expected {from: new, to: contacted}, got %v", decoded)
	}

	// Read back via FindAllByLead too, to prove the round trip survives
	// a fresh SELECT, not just the RETURNING clause of the INSERT.
	list, err := repo.FindAllByLead(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, leadID)
	if err != nil {
		t.Fatalf("find all by lead: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(list))
	}
	var reDecoded map[string]any
	if err := json.Unmarshal(list[0].Metadata, &reDecoded); err != nil {
		t.Fatalf("expected metadata read back via SELECT to be valid JSON, got %q: %v", list[0].Metadata, err)
	}
	if reDecoded["from"] != "new" || reDecoded["to"] != "contacted" {
		t.Errorf("expected {from: new, to: contacted} on re-read, got %v", reDecoded)
	}
}

func TestRepository_Create_NilMetadata_StaysNull(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := activity.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil)

	created, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, activity.CreateInput{
		LeadID: leadID, Type: "note_added",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Metadata != nil {
		t.Errorf("expected nil metadata to stay NULL, got %q", created.Metadata)
	}
}

func TestRepository_Create_CrossOrgLead_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := activity.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	leadInA := seedLead(t, ctx, pool, orgA, nil)

	_, err := repo.Create(ctx, tenant.Context{OrganizationID: orgB, Role: tenant.RoleOwner}, activity.CreateInput{
		LeadID: leadInA, Type: "note_added",
	})
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a lead in a different organization, got: %v", err)
	}
}

func TestRepository_Create_Employee_OtherPersonsLead_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := activity.New(pool)

	org := seedOrganization(t, ctx, pool)
	employeeA := seedMembership(t, ctx, pool, org, "employee-a@example.com", tenant.RoleEmployee)
	employeeB := seedMembership(t, ctx, pool, org, "employee-b@example.com", tenant.RoleEmployee)
	leadForA := seedLead(t, ctx, pool, org, &employeeA)

	_, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &employeeB}, activity.CreateInput{
		LeadID: leadForA, Type: "note_added",
	})
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for an employee acting on someone else's lead, got: %v", err)
	}

	// The assignee themself must succeed.
	if _, err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &employeeA}, activity.CreateInput{
		LeadID: leadForA, Type: "note_added",
	}); err != nil {
		t.Errorf("expected the assignee to be able to add an activity, got: %v", err)
	}
}

func TestRepository_FindAllByLead_NewestFirst(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := activity.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}

	first, err := repo.Create(ctx, tenantCtx, activity.CreateInput{LeadID: leadID, Type: "note_added"})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := repo.Create(ctx, tenantCtx, activity.CreateInput{LeadID: leadID, Type: "call_logged"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	list, err := repo.FindAllByLead(ctx, tenantCtx, leadID)
	if err != nil {
		t.Fatalf("find all by lead: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(list))
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Error("expected newest-first ordering")
	}
}

func TestRepository_FindAllByLead_InvisibleLead_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := activity.New(pool)

	org := seedOrganization(t, ctx, pool)
	other := seedOrganization(t, ctx, pool)
	leadInOther := seedLead(t, ctx, pool, other, nil)

	_, err := repo.FindAllByLead(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, leadInOther)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for a lead in a different organization, got: %v", err)
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

// seedLead inserts a minimal lead row directly via SQL — internal/activity
// doesn't depend on internal/lead, so its tests build the lead they need
// with a raw INSERT rather than importing lead.Repository.
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
