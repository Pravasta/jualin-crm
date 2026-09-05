package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type postgresRepository struct {
	q db.Querier
}

func New(q db.Querier) Repository {
	return &postgresRepository{q: q}
}

// NewNotifier exposes the same concrete implementation as New, typed
// through Notifier — see that interface's doc comment in port.go for
// why this exists (same pattern as auth.NewRefreshTokenRevoker).
func NewNotifier(q db.Querier) Notifier {
	return &postgresRepository{q: q}
}

// Notify inserts one notification for recipientMembershipID. The
// caller is trusted to have already validated the recipient belongs to
// this organization — lead's assignment flow only calls this after its
// own fk_leads_assignee check on the SAME membership id has already
// succeeded, so a second check here would be redundant, not defensive.
func (r *postgresRepository) Notify(ctx context.Context, t tenant.Context, recipientMembershipID uuid.UUID, notifType string, leadID, taskID *uuid.UUID, title string, body *string) error {
	id := uuid.Must(uuid.NewV7())
	const q = `
		INSERT INTO notifications (id, organization_id, recipient_membership_id, type, lead_id, task_id, title, body)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := r.q.Exec(ctx, q, id, t.OrganizationID, recipientMembershipID, notifType, leadID, taskID, title, body); err != nil {
		return fmt.Errorf("notification: notify: %w", err)
	}
	return nil
}

// ExistsThisMonth reports whether notifType already fired for t's
// organization since the start of the current calendar month, cut in
// the ORGANIZATION'S OWN TIMEZONE (Rule #13) — same window
// lead.Repository.CountCreatedThisMonth uses, so "this month" means the
// same thing on both sides of the threshold it gates.
func (r *postgresRepository) ExistsThisMonth(ctx context.Context, t tenant.Context, notifType string) (bool, error) {
	const q = `
		WITH org AS (
			SELECT timezone FROM organizations WHERE id = $1
		)
		SELECT EXISTS (
			SELECT 1 FROM notifications, org
			 WHERE notifications.organization_id = $1
			   AND notifications.type = $2
			   AND notifications.created_at >= date_trunc('month', now() AT TIME ZONE org.timezone) AT TIME ZONE org.timezone
		)`

	var exists bool
	if err := r.q.QueryRow(ctx, q, t.OrganizationID, notifType).Scan(&exists); err != nil {
		return false, fmt.Errorf("notification: exists this month: %w", err)
	}
	return exists, nil
}

// FindAllByRecipient is UNCONDITIONALLY scoped to t.MembershipID,
// regardless of role — this is the one resource in the codebase where
// even Owner/Admin get no broader access (TD §8): a notification
// belongs to exactly one recipient.
func (r *postgresRepository) FindAllByRecipient(ctx context.Context, t tenant.Context, unreadOnly bool) ([]*Notification, error) {
	q := `
		SELECT ` + notificationColumns + `
		FROM notifications
		WHERE organization_id = $1 AND recipient_membership_id = $2`
	if unreadOnly {
		q += ` AND read_at IS NULL`
	}
	q += ` ORDER BY created_at DESC`

	rows, err := r.q.Query(ctx, q, t.OrganizationID, membershipIDOrNil(t))
	if err != nil {
		return nil, fmt.Errorf("notification: find all by recipient: %w", err)
	}
	defer rows.Close()

	var out []*Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("notification: scan: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notification: find all by recipient: %w", err)
	}
	return out, nil
}

// MarkRead treats an already-read notification as success, not an
// error — the client's intent ("I've seen this") is already satisfied.
// Zero rows affected is ambiguous (already read vs. doesn't exist vs.
// belongs to someone else), resolved by a follow-up existence check
// scoped the same way FindAllByRecipient is.
func (r *postgresRepository) MarkRead(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	const q = `
		UPDATE notifications SET read_at = now()
		WHERE id = $1 AND organization_id = $2 AND recipient_membership_id = $3 AND read_at IS NULL`
	tag, err := r.q.Exec(ctx, q, id, t.OrganizationID, membershipIDOrNil(t))
	if err != nil {
		return fmt.Errorf("notification: mark read: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	const existsQ = `
		SELECT EXISTS (
			SELECT 1 FROM notifications
			WHERE id = $1 AND organization_id = $2 AND recipient_membership_id = $3
		)`
	var exists bool
	if err := r.q.QueryRow(ctx, existsQ, id, t.OrganizationID, membershipIDOrNil(t)).Scan(&exists); err != nil {
		return fmt.Errorf("notification: mark read: check existence: %w", err)
	}
	if !exists {
		return httpx.ErrNotFound
	}
	return nil
}

// MarkAllRead is a plain UPDATE — zero rows affected (nothing unread)
// is a legitimate no-op, not an error.
func (r *postgresRepository) MarkAllRead(ctx context.Context, t tenant.Context) error {
	const q = `
		UPDATE notifications SET read_at = now()
		WHERE organization_id = $1 AND recipient_membership_id = $2 AND read_at IS NULL`
	if _, err := r.q.Exec(ctx, q, t.OrganizationID, membershipIDOrNil(t)); err != nil {
		return fmt.Errorf("notification: mark all read: %w", err)
	}
	return nil
}

func membershipIDOrNil(t tenant.Context) uuid.UUID {
	if t.MembershipID == nil {
		return uuid.UUID{}
	}
	return *t.MembershipID
}

const notificationColumns = `
	id, organization_id, recipient_membership_id, type, lead_id, task_id, title, body, read_at, created_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner) (*Notification, error) {
	var n Notification
	err := row.Scan(
		&n.ID, &n.OrganizationID, &n.RecipientMembershipID, &n.Type, &n.LeadID, &n.TaskID,
		&n.Title, &n.Body, &n.ReadAt, &n.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}
