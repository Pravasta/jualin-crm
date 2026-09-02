package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/activity"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/notification"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/webhook"
)

// leadStore is the composition root's implementation of lead.Store —
// same wiring pattern as auth_store.go/membership_store.go/
// invitation_store.go. Its Activity repository comes from
// activity.NewRecorder (#21), its Notification sender from
// notification.NewNotifier (#22), and its Push sender from
// deviceUsecase (Phase 5 #68) — internal/lead never imports any of
// those three packages' domain types, only this file (and those
// packages themselves) know all three exist (ADR-011).
type leadStore struct {
	pool *pgxpool.Pool
	push lead.PushSender
}

// newLeadStore takes push as a parameter, not a package-level default,
// because push genuinely can be nil in tests that construct a leadStore
// directly without caring about push at all — lead.Usecase already
// guards Repos().Push being nil (Phase 5 TD §9.3's doc comment on
// pushAssignmentNotification), so passing nil here is a legitimate,
// supported call, not an oversight.
func newLeadStore(pool *pgxpool.Pool, push lead.PushSender) lead.Store {
	return &leadStore{pool: pool, push: push}
}

func (s *leadStore) InTx(ctx context.Context, fn func(lead.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(leadReposFor(tx, s.push))
	})
}

func (s *leadStore) Repos() lead.Repos {
	return leadReposFor(s.pool, s.push)
}

// leadReposFor's push parameter is deliberately NOT built from q the
// way Lead/Activity/Notification are — push (deviceUsecase) always
// reads/writes through its OWN store, backed by the pool, regardless of
// which querier lead's own transaction is using. That's correct: the
// one caller of Repos().Push (Usecase.pushAssignmentNotification) only
// ever calls it AFTER InTx has already committed (Rule #32), so it
// never needs to participate in lead's transaction at all.
func leadReposFor(q db.Querier, push lead.PushSender) lead.Repos {
	return lead.Repos{
		Lead:         lead.New(q),
		Activity:     activity.NewRecorder(q),
		Notification: notification.NewNotifier(q),
		Push:         push,
		// Built from q, exactly like Activity and unlike Push: the
		// deliveries a lead event produces must commit with that lead or
		// not at all (Phase 7 #101, TD §5). *webhook.Enqueuer satisfies
		// lead.WebhookEnqueuer structurally — this file is the only place
		// that knows both packages exist (ADR-011).
		Webhook: webhook.NewEnqueuer(q),
	}
}
