package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/task"
)

// taskStore is the composition root's implementation of task.Store.
// Its Activity repository comes from activity.NewRecorder —
// internal/task never imports internal/activity's domain types, only
// this file (and internal/activity itself) knows both packages exist
// (ADR-011).
type taskStore struct {
	pool *pgxpool.Pool
}

func newTaskStore(pool *pgxpool.Pool) task.Store {
	return &taskStore{pool: pool}
}

func (s *taskStore) InTx(ctx context.Context, fn func(task.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(taskReposFor(tx))
	})
}

func (s *taskStore) Repos() task.Repos {
	return taskReposFor(s.pool)
}

func taskReposFor(q db.Querier) task.Repos {
	return task.Repos{
		Task:     task.New(q),
		Activity: activity.NewRecorder(q),
	}
}
