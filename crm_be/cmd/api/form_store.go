package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
)

// formStore is the composition root's implementation of form.Store —
// same wiring pattern as apikeyStore.
type formStore struct {
	pool *pgxpool.Pool
}

func newFormStore(pool *pgxpool.Pool) form.Store {
	return &formStore{pool: pool}
}

func (s *formStore) InTx(ctx context.Context, fn func(form.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(formReposFor(tx))
	})
}

func (s *formStore) Repos() form.Repos {
	return formReposFor(s.pool)
}

func formReposFor(q db.Querier) form.Repos {
	return form.Repos{
		Form:  form.New(q),
		Audit: auditlog.New(q),
	}
}
