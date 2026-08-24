package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/customer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
)

// customerStore is the composition root's implementation of
// customer.Store — same wiring pattern as every other _store.go. Its
// Activity repository comes from activity.NewRecorder, same as
// lead/task/membership.
type customerStore struct {
	pool *pgxpool.Pool
}

func newCustomerStore(pool *pgxpool.Pool) customer.Store {
	return &customerStore{pool: pool}
}

func (s *customerStore) InTx(ctx context.Context, fn func(customer.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(customerReposFor(tx))
	})
}

func (s *customerStore) Repos() customer.Repos {
	return customerReposFor(s.pool)
}

func customerReposFor(q db.Querier) customer.Repos {
	return customer.Repos{
		Customer: customer.New(q),
		Activity: activity.NewRecorder(q),
	}
}
