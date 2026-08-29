package device

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

const deviceTokenColumns = `id, organization_id, membership_id, token, platform, created_at, last_seen_at` // #nosec G101 -- a SQL column list, not a credential; gosec's identifier heuristic matches "deviceTokenColumns" as a name

// Upsert relies on uq_device_tokens_token — the same physical device
// registering again (app reopened, token refreshed by FCM, or the
// device handed to a different employee) updates the existing row in
// place rather than erroring or creating a duplicate. id is generated
// fresh by the caller but only used on the INSERT branch; ON CONFLICT
// keeps the existing row's id, which is correct — nothing external
// references a device_tokens.id, so it moving would cost nothing, but
// keeping it stable costs nothing either and is simpler to reason
// about.
func (r *postgresRepository) Upsert(ctx context.Context, t tenant.Context, tok *Token) error {
	const q = `
		INSERT INTO device_tokens (id, organization_id, membership_id, token, platform)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (token) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			membership_id   = EXCLUDED.membership_id,
			platform        = EXCLUDED.platform,
			last_seen_at    = now()
		RETURNING id, created_at, last_seen_at`

	err := r.q.QueryRow(ctx, q,
		tok.ID, t.OrganizationID, tok.MembershipID, tok.Token, tok.Platform,
	).Scan(&tok.ID, &tok.CreatedAt, &tok.LastSeenAt)
	if err != nil {
		return fmt.Errorf("device: upsert: %w", err)
	}
	tok.OrganizationID = t.OrganizationID
	return nil
}

func (r *postgresRepository) FindByToken(ctx context.Context, t tenant.Context, token string) (*Token, error) {
	const q = `
		SELECT ` + deviceTokenColumns + `
		FROM device_tokens
		WHERE token = $1 AND organization_id = $2`

	tok, err := scanRow(r.q.QueryRow(ctx, q, token, t.OrganizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("device: find by token: %w", err)
	}
	return tok, nil
}

func (r *postgresRepository) FindByMembership(ctx context.Context, t tenant.Context, membershipID uuid.UUID) ([]*Token, error) {
	const q = `
		SELECT ` + deviceTokenColumns + `
		FROM device_tokens
		WHERE organization_id = $1 AND membership_id = $2`

	rows, err := r.q.Query(ctx, q, t.OrganizationID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("device: find by membership: %w", err)
	}
	defer rows.Close()

	var out []*Token
	for rows.Next() {
		tok, err := scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("device: scan: %w", err)
		}
		out = append(out, tok)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("device: find by membership: %w", err)
	}
	return out, nil
}

// DeleteByToken is deliberately NOT idempotent-by-design the way
// apikey.Revoke is — zero rows affected is never checked here at all
// (unlike Revoke, which distinguishes "already revoked" from "gone").
// A device token that's already gone by the time this runs (unregistered
// twice, or cleaned up by the push path a moment earlier) is exactly the
// same successful outcome either way: the token doesn't exist anymore.
func (r *postgresRepository) DeleteByToken(ctx context.Context, t tenant.Context, token string) error {
	const q = `DELETE FROM device_tokens WHERE token = $1 AND organization_id = $2`
	if _, err := r.q.Exec(ctx, q, token, t.OrganizationID); err != nil {
		return fmt.Errorf("device: delete by token: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(row rowScanner) (*Token, error) {
	var tok Token
	err := row.Scan(
		&tok.ID, &tok.OrganizationID, &tok.MembershipID, &tok.Token, &tok.Platform,
		&tok.CreatedAt, &tok.LastSeenAt,
	)
	if err != nil {
		return nil, err
	}
	return &tok, nil
}
