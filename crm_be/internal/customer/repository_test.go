package customer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/customer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func TestRepository_Convert_CopiesFieldsAndLeavesLeadUnchanged(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := customer.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil, "won", "Budi Santoso", ptr("budi@example.com"))
	converter := seedMembership(t, ctx, pool, org, "converter@example.com", tenant.RoleOwner)

	created, err := repo.Convert(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, leadID, &converter)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if created.Name != "Budi Santoso" {
		t.Errorf("expected name copied from lead, got %q", created.Name)
	}
	if created.Email == nil || *created.Email != "budi@example.com" {
		t.Errorf("expected email copied from lead, got %v", created.Email)
	}
	if created.ConvertedFromLeadID != leadID {
		t.Errorf("expected converted_from_lead_id %s, got %s", leadID, created.ConvertedFromLeadID)
	}

	// The lead itself must be untouched — still won, still present.
	var leadStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM leads WHERE id = $1`, leadID).Scan(&leadStatus); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if leadStatus != "won" {
		t.Errorf("expected lead to remain won, got %q", leadStatus)
	}
}

func TestRepository_Convert_NotWon_Returns422Sentinel(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := customer.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil, "new", "Budi", nil)

	_, err := repo.Convert(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, leadID, nil)
	if !errors.Is(err, customer.ErrLeadNotWon) {
		t.Fatalf("expected customer.ErrLeadNotWon, got: %v", err)
	}
}

func TestRepository_Convert_LeadNotVisible_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := customer.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	leadInB := seedLead(t, ctx, pool, orgB, nil, "won", "Budi", nil)

	_, err := repo.Convert(ctx, tenant.Context{OrganizationID: orgA, Role: tenant.RoleOwner}, leadInB, nil)
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

// TestRepository_Convert_Twice_BlockedByDatabaseConstraint proves
// uq_customers_org_lead genuinely blocks a second conversion at the
// database level, not just a check the usecase happens to perform once.
func TestRepository_Convert_Twice_BlockedByDatabaseConstraint(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := customer.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil, "won", "Budi", nil)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}

	if _, err := repo.Convert(ctx, tenantCtx, leadID, nil); err != nil {
		t.Fatalf("first convert: %v", err)
	}

	_, err := repo.Convert(ctx, tenantCtx, leadID, nil)
	if !errors.Is(err, customer.ErrAlreadyConverted) {
		t.Fatalf("expected customer.ErrAlreadyConverted, got: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM customers WHERE converted_from_lead_id = $1`, leadID).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 customer row for this lead, got %d", count)
	}
}

func TestRepository_Update_DoesNotChangeOriginatingLead(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := customer.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadID := seedLead(t, ctx, pool, org, nil, "won", "Original Name", nil)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}

	created, err := repo.Convert(ctx, tenantCtx, leadID, nil)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	newName := "Updated Customer Name"
	updated, err := repo.Update(ctx, tenantCtx, created.ID, customer.UpdateInput{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected customer name updated, got %q", updated.Name)
	}

	var leadName string
	if err := pool.QueryRow(ctx, `SELECT name FROM leads WHERE id = $1`, leadID).Scan(&leadName); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if leadName != "Original Name" {
		t.Errorf("expected the lead's name to be untouched, got %q", leadName)
	}
}

func TestRepository_Employee_VisibilityIsThroughOriginatingLead(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := customer.New(pool)

	org := seedOrganization(t, ctx, pool)
	leadOwner := seedMembership(t, ctx, pool, org, "lead-owner@example.com", tenant.RoleEmployee)
	stranger := seedMembership(t, ctx, pool, org, "stranger@example.com", tenant.RoleEmployee)
	leadID := seedLead(t, ctx, pool, org, &leadOwner, "won", "Budi", nil)

	created, err := repo.Convert(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, leadID, nil)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if _, err := repo.FindByID(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &leadOwner}, created.ID); err != nil {
		t.Errorf("expected the lead owner to see the customer, got: %v", err)
	}
	if _, err := repo.FindByID(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleEmployee, MembershipID: &stranger}, created.ID); !errors.Is(err, httpx.ErrNotFound) {
		t.Errorf("expected httpx.ErrNotFound for an unrelated employee, got: %v", err)
	}
}

func ptr(s string) *string { return &s }

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

// seedLead inserts a lead row directly via SQL — internal/customer
// doesn't depend on internal/lead, so its tests build the lead they
// need with a raw INSERT (same pattern activity's/task's tests use).
func seedLead(t *testing.T, ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, assignedTo *uuid.UUID, status, name string, email *string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	var q string
	var args []any
	if status == "lost" {
		q = `INSERT INTO leads (id, organization_id, lead_number, name, email, source, assigned_to_membership_id, status, lost_reason)
			VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), $3, $4, 'manual', $5, $6, 'other')`
		args = []any{id, org, name, email, assignedTo, status}
	} else {
		q = `INSERT INTO leads (id, organization_id, lead_number, name, email, source, assigned_to_membership_id, status)
			VALUES ($1, $2, (SELECT next_lead_number FROM organizations WHERE id = $2), $3, $4, 'manual', $5, $6)`
		args = []any{id, org, name, email, assignedTo, status}
	}
	if _, err := pool.Exec(ctx, q, args...); err != nil {
		t.Fatalf("failed to seed lead: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE organizations SET next_lead_number = next_lead_number + 1 WHERE id = $1`, org); err != nil {
		t.Fatalf("failed to bump next_lead_number: %v", err)
	}
	return id
}
