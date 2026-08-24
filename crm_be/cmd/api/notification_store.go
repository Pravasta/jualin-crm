package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/notification"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
)

// notificationStore is the composition root's implementation of
// notification.Store — same wiring pattern as every other _store.go.
type notificationStore struct {
	pool *pgxpool.Pool
}

func newNotificationStore(pool *pgxpool.Pool) notification.Store {
	return &notificationStore{pool: pool}
}

func (s *notificationStore) InTx(ctx context.Context, fn func(notification.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(notificationReposFor(tx))
	})
}

func (s *notificationStore) Repos() notification.Repos {
	return notificationReposFor(s.pool)
}

func notificationReposFor(q db.Querier) notification.Repos {
	return notification.Repos{
		Notification: notification.New(q),
	}
}
