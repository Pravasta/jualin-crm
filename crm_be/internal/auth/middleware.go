package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const tenantContextKey = "tenant_context"

// AuthMiddleware implements TD phase 1 §7's chain:
// RequestID → Logging → Recovery → [Auth] → [CSRF] → [RBAC] → Handler.
// RBAC is #11; this only establishes tenant.Context and, when the access
// token came from the access_token cookie rather than Authorization:
// Bearer, enforces CSRF for non-GET requests (TD §5).
func AuthMiddleware(usecase *Usecase) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, viaCookie := extractAccessToken(c)
		if raw == "" {
			httpx.RespondError(c, http.StatusUnauthorized, "authentication_required", "Autentikasi diperlukan.")
			c.Abort()
			return
		}

		claims, err := usecase.ParseAccessToken(raw)
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

		userID, membershipID := claims.UserID, claims.MembershipID
		t := tenant.Context{
			OrganizationID: claims.OrganizationID,
			PrincipalType:  tenant.PrincipalUser,
			MembershipID:   &membershipID,
			UserID:         &userID,
			Role:           claims.Role,
			RequestID:      httpx.RequestIDFromContext(c),
		}
		c.Set(tenantContextKey, t)
		c.Next()
	}
}

// TenantFromContext returns the tenant.Context AuthMiddleware built for
// this request. Only safe to call on routes mounted behind
// AuthMiddleware — it panics otherwise, deliberately: a handler reading
// a tenant.Context that was never established is a routing bug, not a
// recoverable condition.
func TenantFromContext(c *gin.Context) tenant.Context {
	v, ok := c.Get(tenantContextKey)
	if !ok {
		panic("auth: TenantFromContext called on a route not behind AuthMiddleware")
	}
	return v.(tenant.Context)
}

const accessTokenCookieName = "access_token"

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
	if cookie, err := c.Cookie(accessTokenCookieName); err == nil && cookie != "" {
		return cookie, true
	}
	return "", false
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
