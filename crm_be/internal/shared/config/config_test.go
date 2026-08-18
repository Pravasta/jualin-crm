package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
)

// unsetEnv removes an env var for the duration of the test and restores its
// previous value afterward. t.Setenv cannot express "unset" — setting a var
// to "" still counts as present for env's `required` tag, which only checks
// os.LookupEnv's ok, not the value.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			if err := os.Setenv(key, orig); err != nil {
				t.Errorf("failed to restore %s: %v", key, err)
			}
		}
	})
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	unsetEnv(t, "DATABASE_URL")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing, got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("expected error to mention DATABASE_URL, got: %v", err)
	}
}

func TestLoad_InvalidAppEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("APP_ENV", "prod")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid APP_ENV, got nil")
	}
	if !strings.Contains(err.Error(), "APP_ENV") || !strings.Contains(err.Error(), "prod") {
		t.Errorf("expected error to mention APP_ENV and the offending value, got: %v", err)
	}
}

func TestLoad_InvalidDBMaxConns(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("DB_MAX_CONNS", "5000")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for out-of-range DB_MAX_CONNS, got nil")
	}
	if !strings.Contains(err.Error(), "DB_MAX_CONNS") {
		t.Errorf("expected error to mention DB_MAX_CONNS, got: %v", err)
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid LOG_LEVEL, got nil")
	}
	if !strings.Contains(err.Error(), "LOG_LEVEL") {
		t.Errorf("expected error to mention LOG_LEVEL, got: %v", err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Errorf("expected default AppEnv=development, got %q", cfg.AppEnv)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("expected default HTTPPort=8080, got %d", cfg.HTTPPort)
	}
	if cfg.IsProduction() {
		t.Error("expected IsProduction() to be false in development")
	}
}

func TestLoad_ProductionMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("APP_ENV", "production")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsProduction() {
		t.Error("expected IsProduction() to be true")
	}
}
