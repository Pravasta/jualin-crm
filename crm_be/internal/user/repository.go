package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

// Repository is the one repository in this codebase that deliberately
// does NOT take tenant.Context. users has no organization_id — a user
// can hold memberships in more than one organization (ADR-007), so
// scoping lookups to a single tenant here would be wrong, not just
// unnecessary. Every method below carries its own explanation for why
// it's safe to skip tenant scoping, rather than leaving that as an
// implicit assumption a later reader has to rediscover.
type Repository struct {
	q db.Querier
}

func New(q db.Querier) *Repository {
	return &Repository{q: q}
}

// FindByEmail looks up a user across the entire system. Safe without
// tenant scoping: this is exactly the global lookup registration and
// login need — email uniqueness is global by design (freeze bagian 7,
// B1), so there is no tenant boundary to violate here.
func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, email_verified_at, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`

	u, err := scanOne(r.q.QueryRow(ctx, q, normalizeEmail(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user: find by email: %w", err)
	}
	return u, nil
}

// FindByID looks up a user by their global id. Safe without tenant
// scoping for the same reason as FindByEmail — the caller (e.g. an
// authenticated request's JWT `sub` claim) already establishes which
// user this is; there is no second user this could be confused with.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, email_verified_at, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`

	u, err := scanOne(r.q.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user: find by id: %w", err)
	}
	return u, nil
}

// Create inserts a new user. Safe without tenant scoping: creating a
// global identity has no organization to scope to yet — that's exactly
// why this table exists separately from memberships.
func (r *Repository) Create(ctx context.Context, id uuid.UUID, email, passwordHash, fullName string) (*User, error) {
	const q = `
		INSERT INTO users (id, email, password_hash, full_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, full_name, email_verified_at, created_at, updated_at, deleted_at`

	u, err := scanOne(r.q.QueryRow(ctx, q, id, normalizeEmail(email), passwordHash, fullName))
	if err != nil {
		return nil, fmt.Errorf("user: create: %w", err)
	}
	return u, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func scanOne(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
