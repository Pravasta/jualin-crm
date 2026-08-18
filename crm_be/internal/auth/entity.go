package auth

import (
	"time"

	"github.com/google/uuid"
)

// EmailVerificationToken is global, like user.User — it belongs to a
// user_id, not an organization. See email_verification_tokens in
// migration 0002. Unlike user/organization/membership, this entity has
// no consumers outside internal/auth, so its interface (port.go) and
// implementation (repository_postgres.go) both live here.
type EmailVerificationToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

const emailVerificationTTL = 24 * time.Hour

// RegisterInput is Usecase.Register's argument — a usecase-level type,
// not a database entity, but it lives here alongside EmailVerificationToken
// as this package's domain vocabulary.
type RegisterInput struct {
	OrganizationName string
	FullName         string
	Email            string
	Password         string
}

type RegisterOutput struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}
