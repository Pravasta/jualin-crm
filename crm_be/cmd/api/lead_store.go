package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/notification"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
)

// leadStore is the composition root's implementation of lead.Store —
// same wiring pattern as auth_store.go/membership_store.go/
// invitation_store.go. Its Activity repository comes from
// activity.NewRecorder (#21) and its Notification sender from
// notification.NewNotifier (#22) — internal/lead never imports either
// package's domain types, only this file (and those packages
// themselves) know all three exist (ADR-011).
type leadStore struct {
	pool *pgxpool.Pool
}

func newLeadStore(pool *pgxpool.Pool) lead.Store {
	return &leadStore{pool: pool}
}

func (s *leadStore) InTx(ctx context.Context, fn func(lead.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(leadReposFor(tx))
	})
}

func (s *leadStore) Repos() lead.Repos {
	return leadReposFor(s.pool)
}

func leadReposFor(q db.Querier) lead.Repos {
	return lead.Repos{
		Lead:         lead.New(q),
		Activity:     activity.NewRecorder(q),
		Notification: notification.NewNotifier(q),
	}
}
