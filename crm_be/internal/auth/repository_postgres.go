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

// postgresResetRepository is password_reset_tokens' implementation — same
// reasoning as postgresVerificationRepository: no consumer outside
// internal/auth, so no separate package for it.
type postgresResetRepository struct {
	q db.Querier
}

func NewPasswordResetRepository(q db.Querier) PasswordResetTokenRepository {
	return &postgresResetRepository{q: q}
}

func (r *postgresResetRepository) Create(ctx context.Context, id, userID uuid.UUID, tokenHash string) error {
	const q = `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`

	_, err := r.q.Exec(ctx, q, id, userID, tokenHash, time.Now().Add(passwordResetTTL))
	if err != nil {
		return fmt.Errorf("auth: create password reset token: %w", err)
	}
	return nil
}

// FindValidByHash mirrors postgresVerificationRepository.FindValidByHash
// exactly — missing, expired, and already-used tokens are all
// indistinguishable (httpx.ErrNotFound) so the usecase layer can't leak
// which case applies through response differences.
func (r *postgresResetRepository) FindValidByHash(ctx context.Context, hash string) (*PasswordResetToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM password_reset_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()`

	var t PasswordResetToken
	err := r.q.QueryRow(ctx, q, hash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: find password reset token: %w", err)
	}
	return &t, nil
}

func (r *postgresResetRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE password_reset_tokens SET used_at = now() WHERE id = $1`
	_, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("auth: mark password reset token used: %w", err)
	}
	return nil
}

// postgresRefreshTokenRepository is refresh_tokens' implementation — same
// no-consumer-elsewhere reasoning as the two token repositories above.
type postgresRefreshTokenRepository struct {
	q db.Querier
}

func NewRefreshTokenRepository(q db.Querier) RefreshTokenRepository {
	return &postgresRefreshTokenRepository{q: q}
}

func (r *postgresRefreshTokenRepository) Create(ctx context.Context, rt *RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, organization_id, membership_id, token_hash, family_id, client, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.q.Exec(ctx, q, rt.ID, rt.OrganizationID, rt.MembershipID, rt.TokenHash, rt.FamilyID, rt.Client, rt.ExpiresAt)
	if err != nil {
		return fmt.Errorf("auth: create refresh token: %w", err)
	}
	return nil
}

// FindByHashForUpdate locks the row for the duration of the caller's
// transaction (TD phase 1 §15) — two concurrent refresh calls racing on
// the same token hash must serialize, not both observe "not yet
// rotated" and both succeed. It intentionally does not filter out
// expired/revoked/replaced rows: the usecase layer needs to see those
// states to distinguish "expired" from "reuse detected" and react
// differently (plain 401 vs. revoking the whole family).
func (r *postgresRefreshTokenRepository) FindByHashForUpdate(ctx context.Context, hash string) (*RefreshToken, error) {
	const q = `
		SELECT id, organization_id, membership_id, token_hash, family_id, client, expires_at, revoked_at, replaced_by_id, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE`

	rt, err := scanRefreshToken(r.q.QueryRow(ctx, q, hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: find refresh token: %w", err)
	}
	return rt, nil
}

func (r *postgresRefreshTokenRepository) MarkReplaced(ctx context.Context, id, replacedByID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET replaced_by_id = $1 WHERE id = $2`
	_, err := r.q.Exec(ctx, q, replacedByID, id)
	if err != nil {
		return fmt.Errorf("auth: mark refresh token replaced: %w", err)
	}
	return nil
}

// RevokeFamily is the reuse-detection response (TD §4): every token ever
// issued from the same login, not just the one that was replayed, stops
// working immediately.
func (r *postgresRefreshTokenRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`
	_, err := r.q.Exec(ctx, q, familyID)
	if err != nil {
		return fmt.Errorf("auth: revoke refresh token family: %w", err)
	}
	return nil
}

func (r *postgresRefreshTokenRepository) RevokeByID(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("auth: revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllByUserID has no organization_id parameter on purpose — see
// the exception comment on the RefreshTokenRepository interface in
// port.go. Password reset must end every session for this user across
// every organization they belong to, found here via a join through
// memberships since refresh_tokens itself has no user_id column.
func (r *postgresRefreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error {
	const q = `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE revoked_at IS NULL
		  AND membership_id IN (SELECT id FROM memberships WHERE user_id = $1)`

	_, err := r.q.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("auth: revoke all refresh tokens for user: %w", err)
	}
	return nil
}

type refreshTokenScanner interface {
	Scan(dest ...any) error
}

func scanRefreshToken(row refreshTokenScanner) (*RefreshToken, error) {
	var rt RefreshToken
	err := row.Scan(
		&rt.ID, &rt.OrganizationID, &rt.MembershipID, &rt.TokenHash, &rt.FamilyID,
		&rt.Client, &rt.ExpiresAt, &rt.RevokedAt, &rt.ReplacedByID, &rt.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rt, nil
}
