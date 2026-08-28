package httpx

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
)

// SetRateLimitHeaders writes the four rate-limit headers Phase 4's
// public API must send on every response along a rate-limited path
// (TD phase 4 §6 — "dikirim sejak versi pertama", api.md's own
// convention). Retry-After is set ONLY when r.Allowed is false; api.md
// documents it as appearing "hanya pada 429". This writes headers, not
// the response body — call it before deciding what status/body to send,
// so the headers land on both the success and the rejection path.
func SetRateLimitHeaders(c *gin.Context, r ratelimit.Result) {
	c.Header("X-RateLimit-Limit", strconv.Itoa(r.Limit))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(r.Remaining))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(r.ResetAt.Unix(), 10))
	if !r.Allowed {
		retryAfter := int(time.Until(r.ResetAt).Seconds())
		if retryAfter < 0 {
			retryAfter = 0
		}
		c.Header("Retry-After", strconv.Itoa(retryAfter))
	}
}
