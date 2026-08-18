package db_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
)

// globalOrRootTables are excluded from the tenant-scoping requirements
// below — they are not tenant-scoped tables (docs/architecture/freeze.md
// bagian 1.1):
//   - organizations is the tenant root; it has no organization_id
//     because it IS the tenant.
//   - users, email_verification_tokens, password_reset_tokens are global
//     identity — deliberately without organization_id (ADR-007).
//   - goose_db_version is goose's own bookkeeping table, not part of the
//     application schema.
var globalOrRootTables = []string{
	"organizations",
	"users",
	"email_verification_tokens",
	"password_reset_tokens",
	"goose_db_version",
}

// TestCatalog_TenantScopedTablesHaveOrganizationID is the automatic
// enforcer of Rule #1: every tenant-scoped table has organization_id.
// Once written, it catches every future table that forgets this
// convention — forever, without anyone needing to remember to check
// during review. See docs/architecture/freeze.md bagian 8.5 test #5.
func TestCatalog_TenantScopedTablesHaveOrganizationID(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	for _, table := range tenantScopedTables(t, ctx, pool) {
		t.Run(table, func(t *testing.T) {
			var nullable string
			err := pool.QueryRow(ctx, `
				SELECT is_nullable FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'organization_id'`,
				table,
			).Scan(&nullable)
			if err != nil {
				t.Fatalf("table %q has no organization_id column (Rule #1 violation): %v", table, err)
			}
			if nullable != "NO" {
				t.Errorf("table %q's organization_id must be NOT NULL, got nullable=%s", table, nullable)
			}
		})
	}
}

// TestCatalog_TenantScopedTablesHaveCompositeUniqueConstraint is the
// automatic enforcer of Rule #2: every tenant-scoped table has
// UNIQUE (id, organization_id) — the constraint every composite FK
// elsewhere in the database depends on being able to reference.
func TestCatalog_TenantScopedTablesHaveCompositeUniqueConstraint(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	for _, table := range tenantScopedTables(t, ctx, pool) {
		t.Run(table, func(t *testing.T) {
			if !hasCompositeUnique(t, ctx, pool, table) {
				t.Errorf("table %q is missing UNIQUE (id, organization_id) — required by Rule #2 for every tenant-scoped table", table)
			}
		})
	}
}

// tenantScopedTables returns every base table in the public schema minus
// globalOrRootTables — i.e. every table these catalog tests hold to
// Rule #1 and #2.
func tenantScopedTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []string {
	t.Helper()

	rows, err := pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		t.Fatalf("failed to list tables: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan table name: %v", err)
		}
		if !slices.Contains(globalOrRootTables, name) {
			out = append(out, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to list tables: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected at least one tenant-scoped table — did the migration not apply?")
	}
	return out
}

// hasCompositeUnique reports whether table has a UNIQUE constraint whose
// columns are exactly {id, organization_id}, regardless of declaration
// order.
func hasCompositeUnique(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) bool {
	t.Helper()

	// Fetch distinct UNIQUE constraint names on this table first, then
	// look up each one's column set — a constraint can cover more than
	// one column, so grouping happens per constraint, not per row.
	names, err := pool.Query(ctx, `
		SELECT DISTINCT constraint_name FROM information_schema.table_constraints
		WHERE table_schema = 'public' AND table_name = $1 AND constraint_type = 'UNIQUE'`,
		table,
	)
	if err != nil {
		t.Fatalf("failed to list unique constraints for %q: %v", table, err)
	}
	defer names.Close()

	var constraintNames []string
	for names.Next() {
		var n string
		if err := names.Scan(&n); err != nil {
			t.Fatalf("failed to scan constraint name: %v", err)
		}
		constraintNames = append(constraintNames, n)
	}
	if err := names.Err(); err != nil {
		t.Fatalf("failed to list unique constraints for %q: %v", table, err)
	}

	for _, name := range constraintNames {
		cols := constraintColumns(t, ctx, pool, name)
		slices.Sort(cols)
		if slices.Equal(cols, []string{"id", "organization_id"}) {
			return true
		}
	}
	return false
}

func constraintColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool, constraintName string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT column_name FROM information_schema.key_column_usage
		WHERE table_schema = 'public' AND constraint_name = $1`,
		constraintName,
	)
	if err != nil {
		t.Fatalf("failed to fetch columns for constraint %q: %v", constraintName, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("failed to scan column name: %v", err)
		}
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to fetch columns for constraint %q: %v", constraintName, err)
	}
	return cols
}
