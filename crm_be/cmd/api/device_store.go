package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/device"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
)

// deviceStore is the composition root's implementation of device.Store —
// same wiring pattern as apikeyStore/invitationStore.
type deviceStore struct {
	pool *pgxpool.Pool
}

func newDeviceStore(pool *pgxpool.Pool) device.Store {
	return &deviceStore{pool: pool}
}

func (s *deviceStore) InTx(ctx context.Context, fn func(device.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(deviceReposFor(tx))
	})
}

func (s *deviceStore) Repos() device.Repos {
	return deviceReposFor(s.pool)
}

func deviceReposFor(q db.Querier) device.Repos {
	return device.Repos{
		DeviceToken: device.New(q),
	}
}
