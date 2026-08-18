// Package user holds the global identity — a User exists independently
// of any organization. Keeping it global (rather than scoped) is what
// lets one email be invited into a second organization without creating
// a second account (ADR-007).
package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    string
	FullName        string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}
