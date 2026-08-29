// Package device owns device_tokens — one row per app installation per
// physical device, registered so internal/shared/push can reach it, and
// bridges lead's PushSender interface at the composition root (Phase 5
// #68). See migrations/0006_device_tokens.sql for why token is unique
// GLOBALLY rather than per organization.
package device

import (
	"time"

	"github.com/google/uuid"
)

type Token struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	MembershipID   uuid.UUID
	Token          string
	Platform       string
	CreatedAt      time.Time
	LastSeenAt     time.Time
}

// validPlatforms mirrors ck_device_tokens_platform exactly — kept here
// as the one place Go code needs to agree with the CHECK constraint,
// same pattern as apikey.ScopeLeadsWrite mirroring ck_api_keys_scopes.
var validPlatforms = map[string]bool{
	"android": true,
	"ios":     true,
}

// IsValidPlatform reports whether p is a platform value the database
// will accept — checked in Usecase before the INSERT is even attempted,
// so a bad value from a client is a 400 (Rule #33's validation_failed
// shape), not a database constraint violation surfacing as a 500.
func IsValidPlatform(p string) bool {
	return validPlatforms[p]
}
