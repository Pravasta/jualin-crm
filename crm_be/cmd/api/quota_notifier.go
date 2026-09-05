package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/membership"
	"github.com/Pravasta/jualin-crm/crm_be/internal/notification"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// notifTypePlanQuotaExceeded must match migration 0010's
// ck_notifications_type entry exactly. Kept as one literal in this one
// file — the only place that needs to write it — rather than exported
// from either internal/notification or internal/subscription, neither
// of which has a reason to know this string exists.
const notifTypePlanQuotaExceeded = "plan_quota_exceeded"

// quotaNotifier is the composition root's implementation of
// lead.QuotaNotifier (#123) — it needs both internal/membership (who is
// an Owner) and internal/notification (how to tell them), so it lives
// here rather than as a method on either domain's own Usecase (ADR-011:
// neither package has a reason to import the other).
type quotaNotifier struct {
	notifications notification.Repository
	notifier      notification.Notifier
	memberships   membership.Repository
}

func newQuotaNotifier(pool *pgxpool.Pool) *quotaNotifier {
	return &quotaNotifier{
		notifications: notification.New(pool),
		notifier:      notification.NewNotifier(pool),
		memberships:   membership.New(pool),
	}
}

// NotifyQuotaExceededOnce checks the threshold BEFORE finding owners —
// the common case (already notified this month) costs one query, not
// one query plus N inserts skipped.
func (n *quotaNotifier) NotifyQuotaExceededOnce(ctx context.Context, t tenant.Context) error {
	already, err := n.notifications.ExistsThisMonth(ctx, t, notifTypePlanQuotaExceeded)
	if err != nil {
		return fmt.Errorf("quota notifier: check threshold: %w", err)
	}
	if already {
		return nil
	}

	owners, err := n.memberships.FindActiveOwnerIDs(ctx, t)
	if err != nil {
		return fmt.Errorf("quota notifier: find owners: %w", err)
	}

	const title = "Kuota lead bulanan habis"
	body := "Paket Anda sudah mencapai batas lead bulan ini. Lead dari formulir publik tetap masuk apa adanya, tapi Anda tidak bisa menambah kanal baru atau mengundang anggota sampai naik paket."
	for _, ownerID := range owners {
		if err := n.notifier.Notify(ctx, t, ownerID, notifTypePlanQuotaExceeded, nil, nil, title, &body); err != nil {
			return fmt.Errorf("quota notifier: notify owner %s: %w", ownerID, err)
		}
	}
	return nil
}
