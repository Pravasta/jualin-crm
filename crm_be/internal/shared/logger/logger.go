// Package logger configures the application's structured logger.
//
// Loggers are retrieved from context (see httpx.Logger), not from a global
// variable, so request-scoped fields like request_id are always attached
// automatically.
package logger

import (
	"log/slog"
	"os"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
)

// New builds the root slog.Logger for the process. In production it emits
// JSON; in development it emits human-readable text.
//
// Per Rule #26: never log passwords, raw API keys, tokens, or full lead
// payloads. Redaction belongs in the logger/call site that handles that
// domain, not here — this is the generic setup only.
func New(cfg *config.Config) *slog.Logger {
	level := parseLevel(cfg.LogLevel)

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
