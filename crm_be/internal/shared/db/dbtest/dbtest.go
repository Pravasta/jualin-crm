// Package dbtest provides a real PostgreSQL instance for tests, via
// testcontainers. Repositories are tested against real PostgreSQL, never a
// mock — a mock database would never catch a tenant-isolation bug. See
// .claude/skills/jualin-backend and docs/architecture/multi-tenancy.md
// lapis 4.
//
// This package is deliberately separate from internal/shared/db (which
// cmd/api imports for production use). Keeping testcontainers-go and its
// Docker-client dependencies isolated here means only _test.go files ever
// import dbtest, and `go build ./cmd/api` never links test infrastructure
// into the shipped binary.
package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/Pravasta/jualin-crm/crm_be/migrations"
)

// One container is started per test binary (not per test, not per
// package) to keep the few seconds of container startup off the critical
// path of every individual test. Migrations run once against it; NewPool
// truncates tables before returning, rather than re-migrating, so per-test
// setup stays cheap no matter how many tests share the container.
var (
	containerOnce sync.Once
	containerConn string
	containerErr  error

	migrateOnce sync.Once
	migrateErr  error
)

// ConnString starts (once per process) a postgres testcontainer and
// returns its connection string. Safe to call from multiple packages'
// tests — the container is shared across the whole test binary run.
func ConnString(ctx context.Context) (string, error) {
	containerOnce.Do(func() {
		container, err := tcpostgres.Run(ctx,
			"postgres:17-alpine",
			tcpostgres.WithDatabase("jualin_crm_test"),
			tcpostgres.WithUsername("jualin"),
			tcpostgres.WithPassword("jualin"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			containerErr = fmt.Errorf("dbtest: failed to start postgres testcontainer: %w", err)
			return
		}

		connStr, err := container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			containerErr = fmt.Errorf("dbtest: failed to get connection string: %w", err)
			return
		}
		containerConn = connStr
	})
	return containerConn, containerErr
}

// ensureMigrated applies every migration exactly once per process, using
// the same embedded SQL and goose dialect as cmd/migrate — tests run
// against the identical schema production would have.
func ensureMigrated(dsn string) error {
	migrateOnce.Do(func() {
		sqlDB, err := sql.Open("pgx", dsn)
		if err != nil {
			migrateErr = fmt.Errorf("dbtest: failed to open database for migration: %w", err)
			return
		}
		defer func() {
			// Don't let a close failure mask a more important migration
			// error that already happened.
			if closeErr := sqlDB.Close(); closeErr != nil && migrateErr == nil {
				migrateErr = fmt.Errorf("dbtest: failed to close database: %w", closeErr)
			}
		}()

		goose.SetBaseFS(migrations.FS)
		if err := goose.SetDialect("postgres"); err != nil {
			migrateErr = fmt.Errorf("dbtest: %w", err)
			return
		}
		if err := goose.Up(sqlDB, "."); err != nil {
			migrateErr = fmt.Errorf("dbtest: migration failed: %w", err)
		}
	})
	return migrateErr
}

// NewPool connects to the shared test container, ensures migrations have
// been applied, truncates every table in the public schema, and returns a
// ready-to-use pool. Call once per test — cleanup is registered
// automatically via t.Cleanup.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	dsn, err := ConnString(ctx)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	if err := ensureMigrated(dsn); err != nil {
		t.Fatalf("dbtest: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("dbtest: failed to connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := truncateAll(ctx, pool); err != nil {
		t.Fatalf("dbtest: failed to reset state: %v", err)
	}

	return pool
}

// truncateAll clears every table in the public schema. Migration state
// (goose_db_version) is preserved deliberately — tests reset data, not
// schema.
func truncateAll(ctx context.Context, pool *pgxpool.Pool) error {
	const q = `
		SELECT string_agg(quote_ident(tablename), ', ')
		FROM pg_tables
		WHERE schemaname = 'public' AND tablename != 'goose_db_version'`

	var tables *string
	if err := pool.QueryRow(ctx, q).Scan(&tables); err != nil {
		return fmt.Errorf("failed to list tables for truncate: %w", err)
	}
	if tables == nil {
		return nil // nothing to truncate yet (e.g. Phase 0 has no domain tables)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", *tables)); err != nil {
		return fmt.Errorf("failed to truncate tables: %w", err)
	}
	return nil
}
