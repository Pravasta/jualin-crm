package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InTx runs fn inside a transaction: begin, then commit on success,
// rollback on error, rollback on panic (re-panicking after cleanup).
//
// This is the ONLY way into a transaction in this codebase (Rule #32 —
// see .claude/skills/jualin-backend). Two things depend on that
// exclusivity: external side effects (email, HTTP calls) must never
// happen inside a transaction, and this is the single place a future
// `SET LOCAL app.current_org_id` for row-level security would be
// inserted — see docs/architecture/multi-tenancy.md lapis 3.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err = fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return fmt.Errorf("db: rollback failed: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: failed to commit transaction: %w", err)
	}

	return nil
}
