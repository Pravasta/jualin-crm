// Package db wraps the PostgreSQL connection pool and the transaction
// manager every repository goes through. See docs/architecture/multi-tenancy.md
// lapis 3: this is the single choke point where SET LOCAL for row-level
// security would be inserted later — that is why InTx (tx.go) exists as
// the only way into a transaction, rather than callers managing pgx.Tx
// directly.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
)

// New builds a connection pool from config and verifies connectivity with
// a bounded ping before returning — a pool that cannot reach the database
// should fail at boot, not on the first request.
func New(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: invalid DATABASE_URL: %w", err)
	}
	poolCfg.MaxConns = int32(cfg.DBMaxConns) // #nosec G115 -- bounded to [1,1000] by config.validate()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: failed to create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: failed to reach database: %w", err)
	}

	return pool, nil
}

// Ping checks connectivity within the given timeout. Used by the
// /health/ready endpoint — never by /health, which must not touch the
// database (Rule: liveness vs readiness, see phases/00-foundation/td.md §7).
func Ping(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return pool.Ping(pingCtx)
}
