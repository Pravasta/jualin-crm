package httpx

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

const (
	corsAllowedMethods = "GET, POST, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders = "Content-Type, Authorization, X-CSRF-Token, Idempotency-Key"
	// corsMaxAge caches a preflight result for ten minutes so a PATCH
	// from the dashboard doesn't cost two round trips on every request
	// (Phase 3 TD §1.2).
	corsMaxAge = "600"
)

// CORS answers browser preflight (OPTIONS) and, for an allowed origin,
// echoes it back verbatim — never "*", because
// Access-Control-Allow-Credentials: true and a wildcard origin are
// mutually exclusive (Phase 3 TD §1.1). An origin NOT in allowedOrigins
// gets no CORS headers at all and the request still proceeds: the
// browser is what rejects it client-side, the server never confirms or
// denies which origins are configured.
//
// Must run before authn.Middleware (Phase 3 TD §1.2) — a preflight
// request carries no credentials and must never reach the auth layer.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := origin != "" && slices.Contains(allowedOrigins, origin)

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			// Origin-dependent response — caches must not serve one
			// origin's CORS headers to a different origin's request.
			c.Writer.Header().Set("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			if allowed {
				c.Writer.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
				c.Writer.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
				c.Writer.Header().Set("Access-Control-Max-Age", corsMaxAge)
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
