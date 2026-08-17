package httpx

import (
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	requestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
	loggerKey       = "logger"
)

// RequestID must run first in the middleware chain. It reads X-Request-ID
// from the incoming request (so it can be correlated with an upstream
// reverse proxy) or generates a UUIDv7 (Rule #12), stores it on the gin
// context, and echoes it back in the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			generated, err := uuid.NewV7()
			if err != nil {
				// crypto/rand failure is effectively unrecoverable; fall back
				// to a random v4 rather than leaving the request unidentified.
				generated = uuid.New()
			}
			id = generated.String()
		}

		c.Set(requestIDKey, id)
		c.Writer.Header().Set(requestIDHeader, id)
		c.Next()
	}
}

// RequestIDFromContext returns the request id set by RequestID, or ""
// if the middleware has not run (should not happen in normal operation).
func RequestIDFromContext(c *gin.Context) string {
	id, _ := c.Get(requestIDKey)
	s, _ := id.(string)
	return s
}

// Logging attaches a request-scoped *slog.Logger (carrying request_id) to
// the gin context, then logs one line after the handler completes.
//
// Retrieve the request-scoped logger via Logger(c) rather than closing over
// a package-level logger — that is what makes request_id show up on every
// log line a handler emits, automatically.
func Logging(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqLogger := base.With("request_id", RequestIDFromContext(c))
		c.Set(loggerKey, reqLogger)

		c.Next()

		reqLogger.Info("request completed",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// Logger returns the request-scoped logger set by Logging. If Logging has
// not run, it falls back to slog.Default() so callers never receive nil.
func Logger(c *gin.Context) *slog.Logger {
	if v, ok := c.Get(loggerKey); ok {
		if l, ok := v.(*slog.Logger); ok {
			return l
		}
	}
	return slog.Default()
}

// Recovery must run after Logging so it can use the request-scoped logger.
// A panic is logged with its stack trace and answered with a generic 500 —
// internal details never leak into the response (Rule #26).
func Recovery(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				Logger(c).Error("panic recovered",
					"panic", r,
					"stack", string(debug.Stack()),
				)
				RespondError(c, 500, "internal_error", "Terjadi kesalahan internal.")
				c.Abort()
			}
		}()
		c.Next()
	}
}
