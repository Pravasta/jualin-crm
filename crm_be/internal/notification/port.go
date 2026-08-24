package notification

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is what notification's own Usecase calls — deliberately
// missing a Create method: nothing in this package's own API creates a
// notification (TD §8's endpoint table has no POST /v1/notifications).
// Creation only happens through Notifier below, from another domain's
// transaction.
type Repository interface {
	FindAllByRecipient(ctx context.Context, t tenant.Context, unreadOnly bool) ([]*Notification, error)
	MarkRead(ctx context.Context, t tenant.Context, id uuid.UUID) error
	MarkAllRead(ctx context.Context, t tenant.Context) error
}

// Notifier is the narrow surface other domains (lead, eventually task)
// need to create a notification from inside their own Store.InTx —
// expressed with only primitive/shared types so a consumer's own
// locally-declared NotificationSender interface (ADR-011) is satisfied
// structurally without importing this package. Same bridging role as
// activity.Recorder.
type Notifier interface {
	Notify(ctx context.Context, t tenant.Context, recipientMembershipID uuid.UUID, notifType string, leadID, taskID *uuid.UUID, title string, body *string) error
}

// Repos bundles what a single Usecase call needs — one field today,
// same shape as every other domain's Unit of Work even though
// notification's own usecase never itself needs a multi-repository
// transaction (kept for composition-root wiring consistency).
type Repos struct {
	Notification Repository
}

// Store is the Unit of Work Usecase depends on.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
