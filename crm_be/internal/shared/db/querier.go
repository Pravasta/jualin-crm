package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx. Repositories are
// constructed with a Querier rather than a concrete pool so the exact
// same repository type works whether called directly against the pool or
// from inside an InTx closure — this is what makes atomic multi-table
// writes (e.g. registration creating a user, organization, membership,
// and subscription together) possible without repositories knowing
// anything about transactions.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
