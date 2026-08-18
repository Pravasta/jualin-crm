package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

// postgresVerificationRepository is this package's only repository
// implementation — email_verification_tokens has no consumers outside
// internal/auth, unlike user/organization/membership/subscription, whose
// implementations live in their own packages and are wired in through
// the interfaces in port.go.
type postgresVerificationRepository struct {
	q db.Querier
}

// NewVerificationRepository is exported so the composition root
// (cmd/api) can construct it when assembling Repos — the same reason
// user.New, organization.New etc. are exported from their packages.
func NewVerificationRepository(q db.Querier) VerificationTokenRepository {
	return &postgresVerificationRepository{q: q}
}

func (r *postgresVerificationRepository) Create(ctx context.Context, id, userID uuid.UUID, tokenHash string) error {
	const q = `
		INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`

	_, err := r.q.Exec(ctx, q, id, userID, tokenHash, time.Now().Add(emailVerificationTTL))
	if err != nil {
		return fmt.Errorf("auth: create verification token: %w", err)
	}
	return nil
}

// FindValidByHash returns the token row only if it exists, is unexpired,
// and hasn't been used — any other case maps to the same
// httpx.ErrNotFound so a caller can't distinguish "wrong token" from
// "expired token" from "already used token" through timing or response
// shape (all three become invalid_token at the usecase layer).
func (r *postgresVerificationRepository) FindValidByHash(ctx context.Context, hash string) (*EmailVerificationToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM email_verification_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()`

	var t EmailVerificationToken
	err := r.q.QueryRow(ctx, q, hash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: find verification token: %w", err)
	}
	return &t, nil
}

func (r *postgresVerificationRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE email_verification_tokens SET used_at = now() WHERE id = $1`
	_, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("auth: mark verification token used: %w", err)
	}
	return nil
}
