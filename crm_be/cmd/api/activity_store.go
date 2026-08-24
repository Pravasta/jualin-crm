package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
)

// activityStore is the composition root's implementation of
// activity.Store — same wiring pattern as lead_store.go.
type activityStore struct {
	pool *pgxpool.Pool
}

func newActivityStore(pool *pgxpool.Pool) activity.Store {
	return &activityStore{pool: pool}
}

func (s *activityStore) InTx(ctx context.Context, fn func(activity.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(activityReposFor(tx))
	})
}

func (s *activityStore) Repos() activity.Repos {
	return activityReposFor(s.pool)
}

func activityReposFor(q db.Querier) activity.Repos {
	return activity.Repos{
		Activity: activity.New(q),
	}
}
