package auditlog

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

type Repository struct {
	q db.Querier
}

func New(q db.Querier) *Repository {
	return &Repository{q: q}
}

// Record inserts one audit log entry. It has no failed-login counterpart
// on purpose — audit_logs.organization_id is NOT NULL, and a failed
// login attempt has no tenant to attach to yet (freeze bagian 7,
// keputusan default). Failed logins go to the application logger, not
// here.
func (r *Repository) Record(
	ctx context.Context,
	t tenant.Context,
	actorMembershipID *uuid.UUID,
	action string,
) error {
	id := uuid.Must(uuid.NewV7())

	const q = `
		INSERT INTO audit_logs (id, organization_id, actor_type, actor_membership_id, action, request_id)
		VALUES ($1, $2, $3, $4, $5, $6)`

	var requestID *string
	if t.RequestID != "" {
		requestID = &t.RequestID
	}

	_, err := r.q.Exec(ctx, q, id, t.OrganizationID, t.PrincipalType, actorMembershipID, action, requestID)
	if err != nil {
		return fmt.Errorf("auditlog: record: %w", err)
	}
	return nil
}
