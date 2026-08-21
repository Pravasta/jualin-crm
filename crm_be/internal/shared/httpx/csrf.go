package httpx

import (
	"crypto/subtle"

	"github.com/gin-gonic/gin"
)

// CSRFCookieName holds the double-submit CSRF token. It is deliberately
// NOT HttpOnly — client-side JavaScript must be able to read it and echo
// it back in CSRFHeaderName, which is the entire mechanism (TD phase 1
// §5).
const CSRFCookieName = "csrf_token"

// CSRFHeaderName is required on every non-GET request authenticated via
// cookie. Requests authenticated via Authorization: Bearer are exempt —
// a bearer token is never sent automatically by the browser, so it can't
// be forged by a cross-site request the way an ambient cookie can.
const CSRFHeaderName = "X-CSRF-Token"

// VerifyCSRF reports whether the CSRF cookie and header match. It has no
// opinion on *when* the check applies — callers (an auth middleware, or
// a handler that just read a session cookie) decide that based on
// whether the request was authenticated via cookie in the first place.
func VerifyCSRF(c *gin.Context) bool {
	cookieVal, err := c.Cookie(CSRFCookieName)
	if err != nil || cookieVal == "" {
		return false
	}
	header := c.GetHeader(CSRFHeaderName)
	if header == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieVal), []byte(header)) == 1
}
