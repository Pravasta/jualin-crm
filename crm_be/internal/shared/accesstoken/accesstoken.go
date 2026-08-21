// Package accesstoken issues and verifies the short-lived JWT access
// token (TD phase 1 §4). It is a thin wrapper around golang-jwt/jwt/v5 —
// kept separate from internal/shared/token because the two are
// structurally different (a signed, self-contained JWT vs. an opaque
// random value whose hash is looked up in the database) and TD treats
// them as distinct concepts.
//
// organization_id, membership_id, and role travel inside the token's
// signed claims, never as separate client-supplied fields — this is
// Rule #5 enforced cryptographically: a caller cannot change which
// organization a request acts on without forging a valid signature.
package accesstoken

import (
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

var ErrInvalidToken = errors.New("accesstoken: invalid or expired token")

// Claims is the decoded shape of an access token. UserID/OrganizationID/
// MembershipID/Role correspond to TD phase 1 §4's sub/org/mem/role.
type Claims struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	MembershipID   uuid.UUID
	Role           tenant.Role
	jwtlib.RegisteredClaims
}

// Issue signs a new access token valid for ttl.
func Issue(secret []byte, ttl time.Duration, userID, orgID, membershipID uuid.UUID, role tenant.Role) (string, error) {
	jti, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("accesstoken: generate jti: %w", err)
	}

	now := time.Now()
	claims := Claims{
		UserID:         userID,
		OrganizationID: orgID,
		MembershipID:   membershipID,
		Role:           role,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
			ID:        jti.String(),
		},
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("accesstoken: sign: %w", err)
	}
	return signed, nil
}

// Parse verifies raw's signature and expiry and returns its claims.
// Any failure (bad signature, expired, malformed) collapses to
// ErrInvalidToken — callers never need to distinguish the cause, only
// that authentication failed.
func Parse(secret []byte, raw string) (*Claims, error) {
	var claims Claims
	_, err := jwtlib.ParseWithClaims(raw, &claims, func(t *jwtlib.Token) (any, error) {
		return secret, nil
	}, jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Name}))
	if err != nil {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
