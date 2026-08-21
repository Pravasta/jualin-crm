// Package authn establishes tenant.Context from an access token — the
// session-verification middleware every protected endpoint in every
// domain package sits behind. Extracted out of internal/auth in issue
// #11 the moment a second and third real consumer (internal/membership,
// internal/invitation) needed it too (Rule #27) — leaving it in
// internal/auth would force every future protected domain to import
// internal/auth just to check who's logged in, the same God-package
// problem ADR-011 already fixed once for the Store pattern.
package authn

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// ClaimsParser is the one method this package needs from whatever holds
// the JWT secret. *auth.Usecase satisfies this structurally — authn
// never imports internal/auth, so the secret has exactly one owner.
type ClaimsParser interface {
	ParseAccessToken(raw string) (*accesstoken.Claims, error)
}

const tenantContextKey = "tenant_context"

// AccessTokenCookieName is the single source of truth for the cookie
// name both sides of the contract must agree on: auth.Handler sets it at
// login/refresh, this package reads it on every subsequent request.
const AccessTokenCookieName = "access_token"

// Middleware implements TD phase 1 §7's chain:
// RequestID → Logging → Recovery → [Auth] → [CSRF] → [RBAC] → Handler.
// It establishes tenant.Context and, when the access token came from the
// access_token cookie rather than Authorization: Bearer, enforces CSRF
// for non-GET requests (TD §5). Requests without a valid token are
// rejected with 401 — use OptionalMiddleware for routes that accept
// both authenticated and anonymous callers.
func Middleware(parser ClaimsParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, viaCookie := extractAccessToken(c)
		if raw == "" {
			httpx.RespondError(c, http.StatusUnauthorized, "authentication_required", "Autentikasi diperlukan.")
			c.Abort()
			return
		}

		t, err := resolve(parser, raw)
		if err != nil {
			httpx.RespondError(c, http.StatusUnauthorized, "authentication_required", "Token tidak valid atau sudah kedaluwarsa.")
			c.Abort()
			return
		}

		if viaCookie && !isSafeMethod(c.Request.Method) && !httpx.VerifyCSRF(c) {
			httpx.RespondError(c, http.StatusForbidden, "csrf_token_invalid", "Token CSRF tidak valid.")
			c.Abort()
			return
		}

		t.RequestID = httpx.RequestIDFromContext(c)
		c.Set(tenantContextKey, t)
		c.Next()
	}
}

// OptionalMiddleware populates tenant.Context when a valid access token
// is present (cookie or bearer) but never aborts when it's absent or
// invalid — for endpoints TD explicitly marks "publik atau
// terautentikasi" (invitation accept, TD §6.1). It does NOT enforce CSRF
// — a route behind this middleware that also mutates state on an
// authenticated request must check TenantFromContextOK and apply CSRF
// itself if it cares, the same way refresh/logout do for their own
// cookie-sourced credential in #10.
func OptionalMiddleware(parser ClaimsParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, _ := extractAccessToken(c)
		if raw == "" {
			c.Next()
			return
		}
		if t, err := resolve(parser, raw); err == nil {
			t.RequestID = httpx.RequestIDFromContext(c)
			c.Set(tenantContextKey, t)
		}
		c.Next()
	}
}

func resolve(parser ClaimsParser, raw string) (tenant.Context, error) {
	claims, err := parser.ParseAccessToken(raw)
	if err != nil {
		return tenant.Context{}, err
	}
	userID, membershipID := claims.UserID, claims.MembershipID
	return tenant.Context{
		OrganizationID: claims.OrganizationID,
		PrincipalType:  tenant.PrincipalUser,
		MembershipID:   &membershipID,
		UserID:         &userID,
		Role:           claims.Role,
	}, nil
}

// TenantFromContext returns the tenant.Context Middleware built for this
// request. Only safe on routes mounted behind Middleware — it panics
// otherwise, deliberately: a handler reading a tenant.Context that was
// never established is a routing bug, not a recoverable condition.
func TenantFromContext(c *gin.Context) tenant.Context {
	t, ok := TenantFromContextOK(c)
	if !ok {
		panic("authn: TenantFromContext called on a route not behind Middleware")
	}
	return t
}

// TenantFromContextOK is TenantFromContext's non-panicking counterpart,
// for routes behind OptionalMiddleware that must branch on whether the
// caller is authenticated at all.
func TenantFromContextOK(c *gin.Context) (tenant.Context, bool) {
	v, ok := c.Get(tenantContextKey)
	if !ok {
		return tenant.Context{}, false
	}
	t, ok := v.(tenant.Context)
	return t, ok
}

// extractAccessToken prefers Authorization: Bearer (mobile) over the
// access_token cookie (dashboard) — a request presenting both is
// unusual, but bearer is the credential a browser could never send
// automatically, so it's the safer of the two to trust when both exist.
func extractAccessToken(c *gin.Context) (raw string, viaCookie bool) {
	if auth := c.GetHeader("Authorization"); auth != "" {
		const prefix = "Bearer "
		if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
			return auth[len(prefix):], false
		}
	}
	if cookie, err := c.Cookie(AccessTokenCookieName); err == nil && cookie != "" {
		return cookie, true
	}
	return "", false
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
