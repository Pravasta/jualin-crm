package db_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/migrations"
)

// TestMigrationRoundTrip automates what was verified manually in issue #2:
// `up` applies 0001_baseline cleanly, and `down` removes every object it
// created — nothing left behind. dbtest.NewPool already applies migrations
// once for the shared container; this test opens its own connection and
// drives goose directly so it can run `down` without disturbing the
// migrated state every other test in this package depends on.
func TestMigrationRoundTrip(t *testing.T) {
	ctx := context.Background()
	dsn, err := dbtest.ConnString(ctx)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	})

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("failed to set goose dialect: %v", err)
	}

	// Re-apply is a no-op if the shared container is already migrated —
	// goose tracks applied versions in goose_db_version.
	if err := goose.Up(sqlDB, "."); err != nil {
		t.Fatalf("goose up failed: %v", err)
	}
	if !functionExists(t, sqlDB, "set_updated_at") {
		t.Fatal("expected set_updated_at() to exist after migrate up")
	}

	if err := goose.Down(sqlDB, "."); err != nil {
		t.Fatalf("goose down failed: %v", err)
	}
	if functionExists(t, sqlDB, "set_updated_at") {
		t.Fatal("expected set_updated_at() to be gone after migrate down — nothing should be left behind")
	}

	// Leave the shared container migrated for every other test in this
	// (and other) packages that assumes 0001_baseline is applied.
	if err := goose.Up(sqlDB, "."); err != nil {
		t.Fatalf("goose up (restoring state) failed: %v", err)
	}
}

func functionExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM pg_proc WHERE proname = $1)`, name,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check function existence: %v", err)
	}
	return exists
}
