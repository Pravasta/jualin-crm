package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/invitation"
	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/organization"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

// invitationStore is the composition root's implementation of
// invitation.Store — same wiring pattern as authStore/membershipStore.
type invitationStore struct {
	pool *pgxpool.Pool
}

func newInvitationStore(pool *pgxpool.Pool) invitation.Store {
	return &invitationStore{pool: pool}
}

func (s *invitationStore) InTx(ctx context.Context, fn func(invitation.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(invitationReposFor(tx))
	})
}

func (s *invitationStore) Repos() invitation.Repos {
	return invitationReposFor(s.pool)
}

func invitationReposFor(q db.Querier) invitation.Repos {
	return invitation.Repos{
		User:       user.New(q),
		Member:     membership.New(q),
		Org:        organization.New(q),
		Audit:      auditlog.New(q),
		Invitation: invitation.New(q),
	}
}
