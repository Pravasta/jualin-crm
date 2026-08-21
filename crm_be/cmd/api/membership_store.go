package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
)

// membershipStore is the composition root's implementation of
// membership.Store. Its RefreshToken repository comes from
// auth.NewRefreshTokenRevoker — internal/membership never imports
// internal/auth; only this file (and internal/auth itself) knows both
// packages exist (ADR-011).
type membershipStore struct {
	pool *pgxpool.Pool
}

func newMembershipStore(pool *pgxpool.Pool) membership.Store {
	return &membershipStore{pool: pool}
}

func (s *membershipStore) InTx(ctx context.Context, fn func(membership.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(membershipReposFor(tx))
	})
}

func (s *membershipStore) Repos() membership.Repos {
	return membershipReposFor(s.pool)
}

func membershipReposFor(q db.Querier) membership.Repos {
	return membership.Repos{
		Member:       membership.New(q),
		Audit:        auditlog.New(q),
		RefreshToken: auth.NewRefreshTokenRevoker(q),
	}
}
