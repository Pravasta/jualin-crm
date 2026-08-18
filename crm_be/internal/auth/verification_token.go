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

// EmailVerificationToken is global, like user.User — it belongs to a
// user_id, not an organization. See email_verification_tokens in
// migration 0002.
type EmailVerificationToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

const emailVerificationTTL = 24 * time.Hour

// verificationRepository is unexported: email_verification_tokens is an
// implementation detail of this package's Service, not something other
// domains look up directly.
type verificationRepository struct {
	q db.Querier
}

func newVerificationRepository(q db.Querier) *verificationRepository {
	return &verificationRepository{q: q}
}

func (r *verificationRepository) Create(ctx context.Context, id, userID uuid.UUID, tokenHash string) error {
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
// shape (all three become invalid_token at the HTTP layer).
func (r *verificationRepository) FindValidByHash(ctx context.Context, hash string) (*EmailVerificationToken, error) {
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

func (r *verificationRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE email_verification_tokens SET used_at = now() WHERE id = $1`
	_, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("auth: mark verification token used: %w", err)
	}
	return nil
}
