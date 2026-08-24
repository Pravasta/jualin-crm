package notification

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Usecase depends only on Store (port.go), never on *pgxpool.Pool or
// pgx.Tx directly (ADR-011). No authz.Require calls anywhere in this
// package: every method is inherently self-scoped by t.MembershipID in
// the repository — there's no role-based permission question to ask,
// same as GET /v1/me needing no coarse gate.
type Usecase struct {
	store Store
}

func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store}
}

func (u *Usecase) List(ctx context.Context, t tenant.Context, unreadOnly bool) ([]*Notification, error) {
	return u.store.Repos().Notification.FindAllByRecipient(ctx, t, unreadOnly)
}

func (u *Usecase) MarkRead(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	return u.store.Repos().Notification.MarkRead(ctx, t, id)
}

func (u *Usecase) MarkAllRead(ctx context.Context, t tenant.Context) error {
	return u.store.Repos().Notification.MarkAllRead(ctx, t)
}
