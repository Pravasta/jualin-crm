package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

// webhookStore is the composition root's implementation of webhook.Store —
// same wiring pattern as formStore / apikeyStore.
type webhookStore struct {
	pool *pgxpool.Pool
}

func newWebhookStore(pool *pgxpool.Pool) webhook.Store {
	return &webhookStore{pool: pool}
}

func (s *webhookStore) InTx(ctx context.Context, fn func(webhook.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(webhookReposFor(tx))
	})
}

func (s *webhookStore) Repos() webhook.Repos {
	return webhookReposFor(s.pool)
}

func webhookReposFor(q db.Querier) webhook.Repos {
	return webhook.Repos{
		Endpoint: webhook.New(q),
		Delivery: webhook.NewDeliveryRepository(q),
		Audit:    auditlog.New(q),
	}
}
