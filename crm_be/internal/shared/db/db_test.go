package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
)

// Each test creates its own throwaway table rather than relying on a
// migration — these tests verify InTx's transaction mechanics, not
// schema, so the table has no place in migrations/.

func TestInTx_CommitsOnSuccess(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS _intx_test (id int)`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	err = db.InTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO _intx_test (id) VALUES (1)`)
		return err
	})
	if err != nil {
		t.Fatalf("InTx returned error on happy path: %v", err)
	}

	if got := countRows(t, ctx, pool); got != 1 {
		t.Errorf("expected 1 row after commit, got %d", got)
	}
}

func TestInTx_RollsBackOnError(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS _intx_test (id int)`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	wantErr := errors.New("deliberate failure")
	err = db.InTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO _intx_test (id) VALUES (1)`); err != nil {
			return err
		}
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Errorf("expected InTx to return the original error, got: %v", err)
	}
	if got := countRows(t, ctx, pool); got != 0 {
		t.Errorf("expected 0 rows after rollback-on-error, got %d", got)
	}
}

func TestInTx_RollsBackOnPanic(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS _intx_test (id int)`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	func() {
		defer func() {
			r := recover()
			if r != "deliberate panic" {
				t.Errorf("expected panic to propagate after rollback, got: %v", r)
			}
		}()
		_ = db.InTx(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO _intx_test (id) VALUES (1)`); err != nil {
				return err
			}
			panic("deliberate panic")
		})
		t.Error("expected InTx to panic, but it returned normally")
	}()

	if got := countRows(t, ctx, pool); got != 0 {
		t.Errorf("expected 0 rows after rollback-on-panic, got %d", got)
	}
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM _intx_test`).Scan(&n); err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	return n
}
