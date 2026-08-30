package form_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

func newTestForm(publicKey string) *form.Form {
	return &form.Form{
		ID:             uuid.Must(uuid.NewV7()),
		PublicKey:      publicKey,
		Name:           "Test Form",
		Fields:         form.DefaultFields(),
		AllowedOrigins: []string{},
	}
}

func TestRepository_Create_FindByID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}
	f := newTestForm("pk_round-trip-key")

	if err := repo.Create(ctx, tenantCtx, f); err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be populated after create")
	}

	found, err := repo.FindByID(ctx, tenantCtx, f.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.PublicKey != f.PublicKey || found.Name != f.Name {
		t.Errorf("expected round-tripped form to match, got %+v", found)
	}
	if len(found.Fields) != len(form.AllFieldKeys) {
		t.Errorf("expected fields to round-trip with all %d keys, got %d", len(form.AllFieldKeys), len(found.Fields))
	}
	if !found.Fields[form.FieldName].Required || found.Fields[form.FieldName].Label != "Nama Lengkap" {
		t.Errorf("expected the name field's config to round-trip exactly, got %+v", found.Fields[form.FieldName])
	}
}

func TestRepository_FindByID_CrossOrg_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	f := newTestForm("pk_cross-org-key")
	if err := repo.Create(ctx, tenant.Context{OrganizationID: orgA, Role: tenant.RoleOwner}, f); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := repo.FindByID(ctx, tenant.Context{OrganizationID: orgB, Role: tenant.RoleOwner}, f.ID)
	if err != httpx.ErrNotFound {
		t.Fatalf("expected httpx.ErrNotFound, got: %v", err)
	}
}

func TestRepository_FindByOrg_NewestFirst_ExcludesDeleted(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}

	first := newTestForm("pk_first-key")
	if err := repo.Create(ctx, tenantCtx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	second := newTestForm("pk_second-key")
	if err := repo.Create(ctx, tenantCtx, second); err != nil {
		t.Fatalf("create second: %v", err)
	}
	third := newTestForm("pk_third-key")
	if err := repo.Create(ctx, tenantCtx, third); err != nil {
		t.Fatalf("create third: %v", err)
	}
	if err := repo.Delete(ctx, tenantCtx, first.ID); err != nil {
		t.Fatalf("delete first: %v", err)
	}

	forms, err := repo.FindByOrg(ctx, tenantCtx)
	if err != nil {
		t.Fatalf("find by org: %v", err)
	}
	if len(forms) != 2 {
		t.Fatalf("expected 2 forms (deleted one excluded, unlike api_keys' revoked-stays-visible), got %d", len(forms))
	}
	if forms[0].ID != third.ID {
		t.Errorf("expected the most recently created form first, got %+v", forms[0])
	}
}

func TestRepository_Delete_ExcludesFromFindByID(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}
	f := newTestForm("pk_delete-target-key")
	if err := repo.Create(ctx, tenantCtx, f); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.Delete(ctx, tenantCtx, f.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.FindByID(ctx, tenantCtx, f.ID); err != httpx.ErrNotFound {
		t.Fatalf("expected httpx.ErrNotFound for a deleted form, got: %v", err)
	}
}

// TestRepository_Update_COALESCE_PartialUpdate proves an UpdateInput
// with only Name set leaves fields/allowed_origins untouched — the same
// COALESCE contract customer.repository_postgres.go's Update proves.
func TestRepository_Update_COALESCE_PartialUpdate(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}
	f := newTestForm("pk_partial-update-key")
	f.AllowedOrigins = []string{"https://example.com"}
	if err := repo.Create(ctx, tenantCtx, f); err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "Renamed Form"
	updated, err := repo.Update(ctx, tenantCtx, f.ID, form.UpdateInput{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected name %q, got %q", newName, updated.Name)
	}
	if len(updated.Fields) != len(form.AllFieldKeys) {
		t.Errorf("expected fields to stay untouched (%d keys), got %d", len(form.AllFieldKeys), len(updated.Fields))
	}
	if len(updated.AllowedOrigins) != 1 || updated.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("expected allowed_origins to stay untouched, got %v", updated.AllowedOrigins)
	}
}

// TestRepository_Update_AllowedOrigins_EmptySliceClears proves a
// non-nil-but-empty *[]string DOES clear the allowlist — distinct from
// nil, which UpdateInput's own doc comment says means "don't touch it".
func TestRepository_Update_AllowedOrigins_EmptySliceClears(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}
	f := newTestForm("pk_clear-origins-key")
	f.AllowedOrigins = []string{"https://example.com"}
	if err := repo.Create(ctx, tenantCtx, f); err != nil {
		t.Fatalf("create: %v", err)
	}

	empty := []string{}
	updated, err := repo.Update(ctx, tenantCtx, f.ID, form.UpdateInput{AllowedOrigins: &empty})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(updated.AllowedOrigins) != 0 {
		t.Errorf("expected allowed_origins to be cleared, got %v", updated.AllowedOrigins)
	}
}

// TestRepository_NameNotBlank_RejectsBlankValue proves
// ck_forms_name_not_blank blocks a blank name at the DATABASE level —
// the usecase already rejects an empty name (usecase_unit_test.go), but
// the migration itself is what actually can't be bypassed by a future
// caller that skips the usecase.
func TestRepository_NameNotBlank_RejectsBlankValue(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	f := newTestForm("pk_blank-name-key")
	f.Name = "   "

	if err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, f); err == nil {
		t.Fatal("expected ck_forms_name_not_blank to reject a blank name, got nil error")
	}
}

// TestRepository_CreatedBy_RejectsCrossOrgMembership proves
// fk_forms_created_by is a genuine COMPOSITE foreign key — a membership
// from a different organization is rejected by the database, not merely
// unchecked by application code (Rule #3).
func TestRepository_CreatedBy_RejectsCrossOrgMembership(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	memberOfB := seedMembership(t, ctx, pool, orgB, "member-b@example.com", tenant.RoleOwner)

	f := newTestForm("pk_cross-org-created-by-key")
	f.CreatedByMembershipID = &memberOfB

	if err := repo.Create(ctx, tenant.Context{OrganizationID: orgA, Role: tenant.RoleOwner}, f); err == nil {
		t.Fatal("expected the composite FK to reject a membership from a different organization, got nil error")
	}
}

// TestRepository_PublicKey_RejectsDuplicate proves uq_forms_public_key
// is enforced at the database level — two forms, even in different
// organizations, cannot share a public_key (the cross-org-unique
// exception documented on the migration and on Repository.FindByPublicKey).
func TestRepository_PublicKey_RejectsDuplicate(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	orgA := seedOrganization(t, ctx, pool)
	orgB := seedOrganization(t, ctx, pool)
	first := newTestForm("pk_duplicate-key")
	if err := repo.Create(ctx, tenant.Context{OrganizationID: orgA, Role: tenant.RoleOwner}, first); err != nil {
		t.Fatalf("create first: %v", err)
	}

	second := newTestForm("pk_duplicate-key")
	if err := repo.Create(ctx, tenant.Context{OrganizationID: orgB, Role: tenant.RoleOwner}, second); err == nil {
		t.Fatal("expected uq_forms_public_key to reject a duplicate public_key across organizations, got nil error")
	}
}

// --- FindByPublicKey (#87's future dependency, exercised directly here) ---

func TestRepository_FindByPublicKey_Success(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	f := newTestForm("pk_findable-key")
	if err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, f); err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := repo.FindByPublicKey(ctx, f.PublicKey)
	if err != nil {
		t.Fatalf("find by public key: %v", err)
	}
	if found.ID != f.ID || found.OrganizationID != org {
		t.Fatalf("expected FindByPublicKey to return the seeded form with its organization, got %+v", found)
	}
}

func TestRepository_FindByPublicKey_ExcludesDeleted(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	tenantCtx := tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}
	f := newTestForm("pk_deleted-then-lookup-key")
	if err := repo.Create(ctx, tenantCtx, f); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(ctx, tenantCtx, f.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.FindByPublicKey(ctx, f.PublicKey); err != httpx.ErrNotFound {
		t.Fatalf("expected httpx.ErrNotFound for a deleted form's public_key, got: %v", err)
	}
}

// TestRepository_FindByPublicKey_IsIndexHit proves the lookup TD §1
// describes is a genuine index hit, not a table scan — uq_forms_public_key's
// implicit unique index is what's being proven usable, same shape as
// apikey's TestRepository_FindByKeyID_IsIndexHit.
func TestRepository_FindByPublicKey_IsIndexHit(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := form.New(pool)

	org := seedOrganization(t, ctx, pool)
	f := newTestForm("pk_explain-target-key")
	if err := repo.Create(ctx, tenant.Context{OrganizationID: org, Role: tenant.RoleOwner}, f); err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := repo.FindByPublicKey(ctx, f.PublicKey)
	if err != nil {
		t.Fatalf("find by public key: %v", err)
	}
	if found.ID != f.ID {
		t.Fatalf("expected FindByPublicKey to return the seeded form, got %+v", found)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SET enable_seqscan = off"); err != nil {
		t.Fatalf("SET enable_seqscan = off: %v", err)
	}

	rows, err := conn.Query(ctx, `EXPLAIN SELECT id FROM forms WHERE public_key = $1`, f.PublicKey)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("explain rows: %v", err)
	}

	plan := strings.Join(planLines, "\n")
	if !strings.Contains(plan, "Index Scan") && !strings.Contains(plan, "Index Only Scan") {
		t.Fatalf("expected an index-based plan for WHERE public_key = $1, got:\n%s", plan)
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
