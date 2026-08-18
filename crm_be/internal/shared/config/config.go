// Package config loads and validates application configuration from
// environment variables. Config is loaded once at boot; invalid config
// fails startup immediately with a message naming the offending variable.
package config

import (
	"fmt"
	"slices"
	"time"

	"github.com/caarlos0/env/v11"
)

var validAppEnvs = []string{"development", "production"}
var validLogLevels = []string{"debug", "info", "warn", "error"}

// "log" is the only provider that exists so far — LogMailer. A real
// provider is added to this list when it's actually implemented, not
// before (Rule #27).
var validMailProviders = []string{"log"}

// Config holds all environment-derived settings for crm_be.
type Config struct {
	AppEnv string `env:"APP_ENV" envDefault:"development"`

	HTTPPort            int           `env:"HTTP_PORT" envDefault:"8080"`
	HTTPReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"10s"`
	HTTPWriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"10s"`
	HTTPShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`

	DatabaseURL string `env:"DATABASE_URL,required"`
	DBMaxConns  int    `env:"DB_MAX_CONNS" envDefault:"10"`

	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// AppBaseURL is where verification/reset/invitation links point —
	// the dashboard's origin, not this API's.
	AppBaseURL   string `env:"APP_BASE_URL" envDefault:"http://localhost:3000"`
	MailFrom     string `env:"MAIL_FROM" envDefault:"no-reply@localhost"`
	MailProvider string `env:"MAIL_PROVIDER" envDefault:"log"`
}

// Load parses environment variables into a Config and validates it.
// Callers should treat a non-nil error as fatal: log it and exit —
// there is no safe default to fall back to for invalid config.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config invalid: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if !slices.Contains(validAppEnvs, c.AppEnv) {
		return fmt.Errorf("config invalid: APP_ENV must be one of %v, got %q", validAppEnvs, c.AppEnv)
	}
	if !slices.Contains(validLogLevels, c.LogLevel) {
		return fmt.Errorf("config invalid: LOG_LEVEL must be one of %v, got %q", validLogLevels, c.LogLevel)
	}
	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("config invalid: HTTP_PORT must be between 1 and 65535, got %d", c.HTTPPort)
	}
	// Upper bound is a sanity ceiling, not a tuned limit — it exists so
	// DBMaxConns safely fits int32 when passed to pgxpool.Config.MaxConns.
	if c.DBMaxConns <= 0 || c.DBMaxConns > 1000 {
		return fmt.Errorf("config invalid: DB_MAX_CONNS must be between 1 and 1000, got %d", c.DBMaxConns)
	}
	if !slices.Contains(validMailProviders, c.MailProvider) {
		return fmt.Errorf("config invalid: MAIL_PROVIDER must be one of %v, got %q", validMailProviders, c.MailProvider)
	}
	return nil
}

// IsProduction reports whether the app is running in production mode.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}
