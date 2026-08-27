package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/apikey"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
)

// apikeyStore is the composition root's implementation of apikey.Store —
// same wiring pattern as invitationStore/customerStore.
type apikeyStore struct {
	pool *pgxpool.Pool
}

func newAPIKeyStore(pool *pgxpool.Pool) apikey.Store {
	return &apikeyStore{pool: pool}
}

func (s *apikeyStore) InTx(ctx context.Context, fn func(apikey.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(apikeyReposFor(tx))
	})
}

func (s *apikeyStore) Repos() apikey.Repos {
	return apikeyReposFor(s.pool)
}

func apikeyReposFor(q db.Querier) apikey.Repos {
	return apikey.Repos{
		APIKey: apikey.New(q),
		Audit:  auditlog.New(q),
	}
}
