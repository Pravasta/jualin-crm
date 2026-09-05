package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/auth"
	"github.com/Pravasta/jualin-crm/crm_be/internal/invitation"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

// authStore is the composition root's implementation of auth.Store — the
// only place *user.Repository, *organization.Repository,
// *membership.Repository, and *subscription.Repository are assembled
// into auth.Repos. internal/auth itself never imports these concrete
// types (ADR-011); it only knows the interfaces it declared in port.go,
// which these types satisfy structurally.
type authStore struct {
	pool *pgxpool.Pool
}

func newAuthStore(pool *pgxpool.Pool) auth.Store {
	return &authStore{pool: pool}
}

func (s *authStore) InTx(ctx context.Context, fn func(auth.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(reposFor(tx))
	})
}

func (s *authStore) Repos() auth.Repos {
	return reposFor(s.pool)
}

// reposFor builds auth.Repos against any db.Querier — the pool for
// non-transactional reads, or a pgx.Tx for the duration of one
// transaction. Every concrete .New(q) constructor already accepts
// db.Querier (issue #8), so the same repository types work in both
// cases without change.
func reposFor(q db.Querier) auth.Repos {
	return auth.Repos{
		User:         user.New(q),
		Org:          organization.New(q),
		Member:       membership.New(q),
		Sub:          subscription.New(q),
		Plan:         subscription.NewUsecase(subscription.New(q)),
		LeadCount:    lead.New(q),
		SeatCount:    membership.New(q),
		PendingSeats: invitation.New(q),
		Verify:       auth.NewVerificationRepository(q),
		Audit:        auditlog.New(q),
		RefreshToken: auth.NewRefreshTokenRepository(q),
		ResetToken:   auth.NewPasswordResetRepository(q),
	}
}
