package auditlog

import (
	"context"
	"encoding/json"
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

// RecordChange is Record's richer sibling — entity_type/entity_id and
// old_values/new_values as JSON, for actions where showing WHAT changed
// matters, not just that something happened. These columns have existed
// since 0002_identity.sql; Phase 8.5 #124 is their first writer.
//
// actorMembershipID is nil for the internal admin surface (#124):
// t.PrincipalType there is tenant.PrincipalSystem, which
// ck_audit_logs_actor_type already allows and which nothing in this
// codebase had a reason to use before now — a bearer-token-authenticated
// caller is not any membership.
func (r *Repository) RecordChange(
	ctx context.Context,
	t tenant.Context,
	actorMembershipID *uuid.UUID,
	action, entityType string,
	entityID uuid.UUID,
	oldValues, newValues any,
) error {
	id := uuid.Must(uuid.NewV7())

	oldJSON, err := json.Marshal(oldValues)
	if err != nil {
		return fmt.Errorf("auditlog: record change: marshal old_values: %w", err)
	}
	newJSON, err := json.Marshal(newValues)
	if err != nil {
		return fmt.Errorf("auditlog: record change: marshal new_values: %w", err)
	}

	const q = `
		INSERT INTO audit_logs (id, organization_id, actor_type, actor_membership_id, action, entity_type, entity_id, old_values, new_values, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	var requestID *string
	if t.RequestID != "" {
		requestID = &t.RequestID
	}

	_, err = r.q.Exec(ctx, q, id, t.OrganizationID, t.PrincipalType, actorMembershipID, action, entityType, entityID, oldJSON, newJSON, requestID)
	if err != nil {
		return fmt.Errorf("auditlog: record change: %w", err)
	}
	return nil
}
