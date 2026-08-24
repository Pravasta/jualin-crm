package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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

// NewRecorder exposes the same concrete implementation as New, typed
// through Recorder — see that interface's doc comment in port.go for
// why this exists (same pattern as auth.NewRefreshTokenRevoker).
func NewRecorder(q db.Querier) Recorder {
	return &postgresRepository{q: q}
}

// Create inserts one activity, but only if leadID is visible to t —
// tenant-scoped always, and additionally restricted to leads assigned
// to t.MembershipID when t.Role is employee (TD §9's rule, re-expressed
// here as a SQL WHERE EXISTS rather than a Go-level call into
// internal/lead: both packages share one Postgres database, so this
// needs no cross-package interface). Zero rows (lead not visible) comes
// back as pgx.ErrNoRows from QueryRow, mapped to httpx.ErrNotFound —
// same as any other cross-org/cross-scope lookup in this codebase
// (Rule #6: never 403 for a resource in someone else's scope).
func (r *postgresRepository) Create(ctx context.Context, t tenant.Context, in CreateInput) (*Activity, error) {
	metadataJSON, err := marshalMetadata(in.Metadata)
	if err != nil {
		return nil, fmt.Errorf("activity: marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO activities (id, organization_id, lead_id, type, actor_membership_id, body, metadata)
		SELECT $1, $2, $3, $4, $5, $6, $7
		WHERE EXISTS (
			SELECT 1 FROM leads
			WHERE id = $3 AND organization_id = $2 AND deleted_at IS NULL
			  AND (NOT $8 OR assigned_to_membership_id = $9)
		)
		RETURNING ` + activityColumns

	id := uuid.Must(uuid.NewV7())
	row := r.q.QueryRow(ctx, q,
		id, t.OrganizationID, in.LeadID, in.Type, in.ActorMembershipID, in.Body, metadataJSON,
		isEmployee(t), membershipIDOrNil(t),
	)
	created, err := scanActivity(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("activity: create: %w", err)
	}
	return created, nil
}

// Record is Recorder's implementation — called only from inside
// lead/task's own Store.InTx, immediately after they've already
// loaded or written that exact lead in the same transaction. The
// WHERE EXISTS guard still runs (it's the same statement as Create) but
// is a formality in that call path, never expected to fail.
func (r *postgresRepository) Record(ctx context.Context, t tenant.Context, leadID uuid.UUID, activityType string, actorMembershipID *uuid.UUID, metadata map[string]any) error {
	_, err := r.Create(ctx, t, CreateInput{
		LeadID:            leadID,
		Type:              activityType,
		ActorMembershipID: actorMembershipID,
		Metadata:          metadata,
	})
	return err
}

// FindAllByLead returns leadID's timeline, newest first (TD §8: "terbaru
// dulu"), scoped the same way Create is — wrong org or (for an
// employee) a lead not assigned to them both look identical to "empty
// timeline" from a pure SELECT, so the visibility check runs
// separately here and yields httpx.ErrNotFound when the lead itself
// isn't visible, rather than a silently empty list that would leak
// nothing but also confirm nothing.
func (r *postgresRepository) FindAllByLead(ctx context.Context, t tenant.Context, leadID uuid.UUID) ([]*Activity, error) {
	visible, err := r.leadVisible(ctx, t, leadID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, httpx.ErrNotFound
	}

	const q = `
		SELECT ` + activityColumns + `
		FROM activities
		WHERE organization_id = $1 AND lead_id = $2
		ORDER BY created_at DESC`

	rows, err := r.q.Query(ctx, q, t.OrganizationID, leadID)
	if err != nil {
		return nil, fmt.Errorf("activity: find all by lead: %w", err)
	}
	defer rows.Close()

	var out []*Activity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, fmt.Errorf("activity: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("activity: find all by lead: %w", err)
	}
	return out, nil
}

func (r *postgresRepository) leadVisible(ctx context.Context, t tenant.Context, leadID uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM leads
			WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL
			  AND (NOT $3 OR assigned_to_membership_id = $4)
		)`
	var visible bool
	if err := r.q.QueryRow(ctx, q, leadID, t.OrganizationID, isEmployee(t), membershipIDOrNil(t)).Scan(&visible); err != nil {
		return false, fmt.Errorf("activity: check lead visibility: %w", err)
	}
	return visible, nil
}

func isEmployee(t tenant.Context) bool {
	return t.Role == tenant.RoleEmployee
}

func membershipIDOrNil(t tenant.Context) uuid.UUID {
	if t.MembershipID == nil {
		return uuid.UUID{}
	}
	return *t.MembershipID
}

func marshalMetadata(m map[string]any) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

const activityColumns = `
	id, organization_id, lead_id, type, actor_membership_id, body, metadata, created_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanActivity(row rowScanner) (*Activity, error) {
	var a Activity
	err := row.Scan(
		&a.ID, &a.OrganizationID, &a.LeadID, &a.Type, &a.ActorMembershipID, &a.Body, &a.Metadata, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
