package invitation

import (
	"context"
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

func (r *postgresRepository) Create(ctx context.Context, inv *Invitation) error {
	const q = `
		INSERT INTO invitations (id, organization_id, email, role, token_hash, invited_by_membership_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.q.Exec(ctx, q,
		inv.ID, inv.OrganizationID, inv.Email, inv.Role, inv.TokenHash, inv.InvitedByMembershipID, inv.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("invitation: create: %w", err)
	}
	return nil
}

// CountPendingSeats counts invitations that could still turn into a
// membership — the other half of the seat meter (Phase 8.5), summed
// with membership.CountActive by the caller.
//
// Note the predicate is STRICTER than FindByOrgPending below: this one
// also excludes expired invitations. An expired invitation can never be
// accepted, so holding a seat for it would charge the customer for
// something that cannot happen — whereas FindByOrgPending deliberately
// still lists them so the screen can show them as expired.
func (r *postgresRepository) CountPendingSeats(ctx context.Context, t tenant.Context) (int, error) {
	const q = `
		SELECT count(*)
		  FROM invitations
		 WHERE organization_id = $1
		   AND accepted_at IS NULL
		   AND revoked_at IS NULL
		   AND expires_at > now()`

	var n int
	if err := r.q.QueryRow(ctx, q, t.OrganizationID).Scan(&n); err != nil {
		return 0, fmt.Errorf("invitation: count pending seats: %w", err)
	}
	return n, nil
}

// FindByOrgPending lists invitations that are still actionable — not yet
// accepted, not revoked. Expired-but-untouched invitations still appear
// (no background job prunes them in Phase 1); the client can tell from
// expires_at.
func (r *postgresRepository) FindByOrgPending(ctx context.Context, t tenant.Context) ([]*Invitation, error) {
	const q = `
		SELECT id, organization_id, email, role, token_hash, invited_by_membership_id, expires_at, accepted_at, revoked_at, created_at
		FROM invitations
		WHERE organization_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.q.Query(ctx, q, t.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("invitation: find by org pending: %w", err)
	}
	defer rows.Close()

	var out []*Invitation
	for rows.Next() {
		inv, err := scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("invitation: scan: %w", err)
		}
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("invitation: find by org pending: %w", err)
	}
	return out, nil
}

// FindByID scopes to t.OrganizationID — an invitation belonging to
// another organization is indistinguishable from one that doesn't exist
// (Rule #6), same as every other tenant-scoped FindByID in this codebase.
func (r *postgresRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Invitation, error) {
	const q = `
		SELECT id, organization_id, email, role, token_hash, invited_by_membership_id, expires_at, accepted_at, revoked_at, created_at
		FROM invitations
		WHERE id = $1 AND organization_id = $2`

	inv, err := scanOne(r.q.QueryRow(ctx, q, id, t.OrganizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("invitation: find by id: %w", err)
	}
	return inv, nil
}

// FindValidByHash is NOT organization-scoped — see the exception
// documented on the Repository interface in port.go. Missing, expired,
// revoked, and already-accepted invitations all need to be
// distinguished by the caller here (unlike verification/reset tokens):
// TD §13 gives "already accepted" its own error code
// (invitation_already_accepted) distinct from "invalid_token", so this
// query intentionally does NOT filter accepted_at/expires_at/revoked_at —
// Usecase.Accept inspects the row itself to pick the right response.
func (r *postgresRepository) FindValidByHash(ctx context.Context, hash string) (*Invitation, error) {
	const q = `
		SELECT id, organization_id, email, role, token_hash, invited_by_membership_id, expires_at, accepted_at, revoked_at, created_at
		FROM invitations
		WHERE token_hash = $1`

	inv, err := scanOne(r.q.QueryRow(ctx, q, hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("invitation: find valid by hash: %w", err)
	}
	return inv, nil
}

func (r *postgresRepository) MarkAccepted(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE invitations SET accepted_at = now() WHERE id = $1`
	_, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("invitation: mark accepted: %w", err)
	}
	return nil
}

func (r *postgresRepository) MarkRevoked(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE invitations SET revoked_at = now() WHERE id = $1`
	_, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("invitation: mark revoked: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOne(row rowScanner) (*Invitation, error) {
	return scanRow(row)
}

func scanRow(row rowScanner) (*Invitation, error) {
	var inv Invitation
	err := row.Scan(
		&inv.ID, &inv.OrganizationID, &inv.Email, &inv.Role, &inv.TokenHash, &inv.InvitedByMembershipID,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.RevokedAt, &inv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}
