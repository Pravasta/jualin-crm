package lead

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is lead's own table interface — postgresRepository
// (repository_postgres.go) satisfies it, but Usecase depends only on
// this interface (ADR-011), which is what makes usecase_unit_test.go's
// fake possible without Docker. Through #19 this was a concrete
// exported struct (Repository) with no interface — the same migration
// internal/membership went through in #11, the moment a real usecase
// exists to declare the interface it needs.
type Repository interface {
	Create(ctx context.Context, t tenant.Context, in CreateInput) (*Lead, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Lead, error)
	FindByIdempotencyKey(ctx context.Context, t tenant.Context, key string) (*Lead, error)
	FindAllByOrg(ctx context.Context, t tenant.Context, filter ListFilter) ([]*Lead, int, error)
	Update(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in UpdateInput) (*Lead, error)
	UpdateStatus(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, status string, lostReason *string) (*Lead, error)
	UpdateAssignment(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, assignedTo *uuid.UUID) (*Lead, error)
	Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error
}

// ActivityRecorder is declared locally per ADR-011 — lead needs only to
// append a system-generated activity, not activity's full domain type.
// Structurally identical to task.ActivityRecorder (both declared
// independently, not shared — three lines, not worth a common package
// for two call sites) and satisfied by activity.NewRecorder's return
// value at the composition root, same bridging pattern
// auth.RefreshTokenRevoker established for membership in #11.
type ActivityRecorder interface {
	Record(ctx context.Context, t tenant.Context, leadID uuid.UUID, activityType string, actorMembershipID *uuid.UUID, metadata map[string]any) error
}

// NotificationSender is declared locally per ADR-011, same shape as
// notification.Notifier — lead needs only to send a notification, not
// notification's full domain type. Satisfied by notification.NewNotifier's
// return value at the composition root, same bridging pattern as
// ActivityRecorder.
type NotificationSender interface {
	Notify(ctx context.Context, t tenant.Context, recipientMembershipID uuid.UUID, notifType string, leadID, taskID *uuid.UUID, title string, body *string) error
}

// Repos bundles what a single Usecase call needs. Activity was added in
// #21 when lead events first needed cross-table atomicity with activity
// rows (TD §10). Notification was added in #22 for assignment (TD §11).
type Repos struct {
	Lead         Repository
	Activity     ActivityRecorder
	Notification NotificationSender
}

// Store is the Unit of Work Usecase depends on — same shape as every
// other domain's. Required even though most of Repository's methods are
// single statements: Create internally needs a transaction (see its doc
// comment in repository_postgres.go and the proof in #19's
// repository_test.go), so Usecase.Create must be able to call
// store.InTx.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}

// OpenLeadRepository is lead's own interface matching membership's
// locally-declared interface of the same shape — exists purely so the
// composition root can hand membership.Repos.OpenLead a value
// satisfying membership's own interface without membership importing
// this package (ADR-011, same bridging pattern as
// auth.RefreshTokenRevoker). NOT part of Repository above: lead's own
// Usecase never calls these — only membership.Usecase.Deactivate does,
// closing the Phase 1 obligation (TD §13) that a membership can't be
// deactivated while it still owns open leads.
type OpenLeadRepository interface {
	CountOpen(ctx context.Context, t tenant.Context, membershipID uuid.UUID) (int, error)
	UnassignOpen(ctx context.Context, t tenant.Context, membershipID uuid.UUID) ([]uuid.UUID, error)
	ReassignOpen(ctx context.Context, t tenant.Context, membershipID, reassignTo uuid.UUID) ([]uuid.UUID, error)
}
