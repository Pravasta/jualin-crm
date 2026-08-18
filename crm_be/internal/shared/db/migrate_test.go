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

// Each migration gets its own round-trip test targeting an explicit
// version with UpTo/DownTo, rather than one test relying on goose.Down
// undoing "the latest migration". goose.Down only ever rolls back
// whichever migration is currently on top — a single shared test would
// silently start testing the wrong migration's down script every time a
// new migration file is added on top. Targeting versions explicitly means
// adding 0003 later doesn't require touching this file at all.
//
// dbtest.NewPool already applies every migration once for the shared
// container. These tests open their own *sql.DB and drive goose directly
// so they can move the schema version around without disturbing the
// fully-migrated state every other test in this package assumes — each
// restores that state via goose.Up before returning.

func TestMigrationRoundTrip_0001Baseline(t *testing.T) {
	sqlDB := openGooseDB(t)

	// Start from zero explicitly — the shared container may already be
	// fully migrated, which would make UpTo(1) a silent no-op and leave
	// this test not actually exercising 0001's up script.
	if err := goose.DownTo(sqlDB, ".", 0); err != nil {
		t.Fatalf("goose down to 0 failed: %v", err)
	}

	if err := goose.UpTo(sqlDB, ".", 1); err != nil {
		t.Fatalf("goose up to 1 failed: %v", err)
	}
	if !functionExists(t, sqlDB, "set_updated_at") {
		t.Fatal("expected set_updated_at() to exist after migrating to version 1")
	}

	if err := goose.DownTo(sqlDB, ".", 0); err != nil {
		t.Fatalf("goose down to 0 failed: %v", err)
	}
	if functionExists(t, sqlDB, "set_updated_at") {
		t.Fatal("expected set_updated_at() to be gone after rolling back to version 0")
	}

	restoreLatest(t, sqlDB)
}

func TestMigrationRoundTrip_0002Identity(t *testing.T) {
	sqlDB := openGooseDB(t)

	// Land on exactly version 1 first, so UpTo(2) below is a real
	// migration of 0002 alone rather than a no-op against an
	// already-fully-migrated shared container.
	if err := goose.DownTo(sqlDB, ".", 1); err != nil {
		t.Fatalf("goose down to 1 failed: %v", err)
	}

	if err := goose.UpTo(sqlDB, ".", 2); err != nil {
		t.Fatalf("goose up to 2 failed: %v", err)
	}
	for _, table := range identityTables {
		if !tableExists(t, sqlDB, table) {
			t.Errorf("expected table %q to exist after migrating to version 2", table)
		}
	}

	if err := goose.DownTo(sqlDB, ".", 1); err != nil {
		t.Fatalf("goose down to 1 failed: %v", err)
	}
	for _, table := range identityTables {
		if tableExists(t, sqlDB, table) {
			t.Errorf("expected table %q to be gone after rolling back to version 1 — nothing should be left behind", table)
		}
	}
	// 0001's function must survive rolling back 0002 — down only undoes
	// its own migration, not everything beneath it.
	if !functionExists(t, sqlDB, "set_updated_at") {
		t.Fatal("expected set_updated_at() (from 0001) to survive rolling back 0002")
	}

	restoreLatest(t, sqlDB)
}

var identityTables = []string{
	"organizations", "users", "memberships", "subscriptions",
	"invitations", "email_verification_tokens", "password_reset_tokens",
	"refresh_tokens", "audit_logs",
}

func openGooseDB(t *testing.T) *sql.DB {
	t.Helper()
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
	return sqlDB
}

// restoreLatest brings the shared container back to the newest migration
// so other tests sharing this container aren't left on a rolled-back
// schema.
func restoreLatest(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	if err := goose.Up(sqlDB, "."); err != nil {
		t.Fatalf("goose up (restoring latest) failed: %v", err)
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

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = $1)`, name,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check table existence: %v", err)
	}
	return exists
}
