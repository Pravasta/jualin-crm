package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
)

// validJWTSecret satisfies the minimum-length requirement so tests that
// aren't specifically exercising JWT_SECRET validation don't fail on it.
const validJWTSecret = "test-jwt-secret-at-least-32-bytes-long"

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
	t.Setenv("JWT_SECRET", validJWTSecret)
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
	t.Setenv("JWT_SECRET", validJWTSecret)
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
	t.Setenv("JWT_SECRET", validJWTSecret)
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
	t.Setenv("JWT_SECRET", validJWTSecret)

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
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8")
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsProduction() {
		t.Error("expected IsProduction() to be true")
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	unsetEnv(t, "JWT_SECRET")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is missing, got nil")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Errorf("expected error to mention JWT_SECRET, got: %v", err)
	}
}

func TestLoad_ShortJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "too-short")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for a JWT_SECRET shorter than 32 bytes, got nil")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Errorf("expected error to mention JWT_SECRET, got: %v", err)
	}
}

// TestLoad_ProductionRequiresCookieSecure is the direct acceptance
// criterion from issue #10: COOKIE_SECURE=false in production must fail
// boot (Rule #36), not merely log a warning — a cookie sent over plain
// HTTP in production is a token leak, not a degraded-mode condition.
func TestLoad_ProductionRequiresCookieSecure(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TRUSTED_PROXIES", "none")
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV=production and COOKIE_SECURE=false, got nil")
	}
	if !strings.Contains(err.Error(), "COOKIE_SECURE") {
		t.Errorf("expected error to mention COOKIE_SECURE, got: %v", err)
	}
}

// TestLoad_ProductionRequiresCORSAllowedOrigins is issue #30's direct
// acceptance criterion (TD phase 3 §1.1, Rule #36): an empty
// CORS_ALLOWED_ORIGINS in production means the dashboard is dead on
// arrival with no server-side error — boot must fail instead.
func TestLoad_ProductionRequiresCORSAllowedOrigins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("TRUSTED_PROXIES", "none")
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	unsetEnv(t, "CORS_ALLOWED_ORIGINS")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV=production and CORS_ALLOWED_ORIGINS is empty, got nil")
	}
	if !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
		t.Errorf("expected error to mention CORS_ALLOWED_ORIGINS, got: %v", err)
	}
}

// TestLoad_ProductionRequiresTrustedProxies is issue #57's direct
// acceptance criterion (Phase 4.5 TD §1, Rule #36): booting in production
// without ever deciding TRUSTED_PROXIES leaves Gin's own default in
// place — which trusts every peer as a proxy and makes every per-IP
// rate limit forgeable (Rule #34). Boot must fail instead of defaulting
// to that.
func TestLoad_ProductionRequiresTrustedProxies(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	unsetEnv(t, "TRUSTED_PROXIES")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV=production and TRUSTED_PROXIES is unset, got nil")
	}
	if !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Errorf("expected error to mention TRUSTED_PROXIES, got: %v", err)
	}
}

// TestLoad_TrustedProxiesNoneIsValidOutsideProduction proves "none" is a
// real, accepted value (not just "happens to be absent") — the literal a
// single-process deployment with no reverse proxy is expected to set.
func TestLoad_TrustedProxiesNoneIsValidOutsideProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("TRUSTED_PROXIES", "none")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.TrustedProxyCIDRs(); got != nil {
		t.Errorf(`expected TrustedProxyCIDRs() to be nil for "none", got %v`, got)
	}
}

// TestLoad_TrustedProxiesRejectsInvalidEntry proves a typo'd CIDR/IP is a
// boot-time error, not a proxy that silently never gets trusted.
func TestLoad_TrustedProxiesRejectsInvalidEntry(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("TRUSTED_PROXIES", "not-an-ip")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for an invalid TRUSTED_PROXIES entry, got nil")
	}
	if !strings.Contains(err.Error(), "TRUSTED_PROXIES") || !strings.Contains(err.Error(), "not-an-ip") {
		t.Errorf("expected error to mention TRUSTED_PROXIES and the offending value, got: %v", err)
	}
}

// TestLoad_TrustedProxiesRejectsNoneMixedWithCIDR proves "none" can't be
// combined with real entries — it means "no proxy", so pairing it with a
// CIDR is a contradiction, not a partial trust list.
func TestLoad_TrustedProxiesRejectsNoneMixedWithCIDR(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("TRUSTED_PROXIES", "none,10.0.0.0/8")

	_, err := config.Load()
	if err == nil {
		t.Fatal(`expected error when TRUSTED_PROXIES mixes "none" with a CIDR, got nil`)
	}
	if !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Errorf("expected error to mention TRUSTED_PROXIES, got: %v", err)
	}
}

// TestLoad_TrustedProxiesAcceptsCIDRList proves a real multi-entry list
// parses and survives round-trip to TrustedProxyCIDRs() unchanged — the
// exact form (*gin.Engine).SetTrustedProxies needs.
func TestLoad_TrustedProxiesAcceptsCIDRList(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8,172.16.0.5")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.0.0.0/8", "172.16.0.5"}
	got := cfg.TrustedProxyCIDRs()
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("expected entry[%d] = %q, got %q", i, v, got[i])
		}
	}
}

// TestLoad_CORSAllowedOrigins_SplitsOnComma proves the envSeparator tag
// actually parses a multi-origin list (production + staging + preview,
// TD §1.1) rather than treating it as one opaque string.
func TestLoad_CORSAllowedOrigins_SplitsOnComma(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,https://staging.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"http://localhost:3000", "https://staging.example.com"}
	if len(cfg.CORSAllowedOrigins) != len(want) {
		t.Fatalf("expected %v, got %v", want, cfg.CORSAllowedOrigins)
	}
	for i, origin := range want {
		if cfg.CORSAllowedOrigins[i] != origin {
			t.Errorf("expected origin[%d] = %q, got %q", i, origin, cfg.CORSAllowedOrigins[i])
		}
	}
}

// TestLoad_ProductionRejectsLogMailProvider is Phase 4.6 decision E2's
// direct acceptance criterion (issue #63): a production process that
// never sends email fails completely silently — no crash, no error, just
// a registration funnel that quietly stops working. Boot must fail
// instead of defaulting to LogMailer, which also writes the verification
// token itself to the log (Rule #26).
func TestLoad_ProductionRejectsLogMailProvider(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TRUSTED_PROXIES", "none")
	t.Setenv("MAIL_PROVIDER", "log")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV=production and MAIL_PROVIDER=log, got nil")
	}
	if !strings.Contains(err.Error(), "MAIL_PROVIDER") {
		t.Errorf("expected error to mention MAIL_PROVIDER, got: %v", err)
	}
}

// TestLoad_SMTPRequiresHost is issue #63's direct acceptance criterion:
// MAIL_PROVIDER=smtp without a host must fail at boot, not at the first
// user's registration attempt.
func TestLoad_SMTPRequiresHost(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("MAIL_PROVIDER", "smtp")
	unsetEnv(t, "SMTP_HOST")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MAIL_PROVIDER=smtp and SMTP_HOST is unset, got nil")
	}
	if !strings.Contains(err.Error(), "SMTP_HOST") {
		t.Errorf("expected error to mention SMTP_HOST, got: %v", err)
	}
}

// TestLoad_ProductionRejectsSMTPWithoutTLS is decision E3: production
// must never send credentials and verification tokens over an
// unencrypted connection.
func TestLoad_ProductionRejectsSMTPWithoutTLS(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TRUSTED_PROXIES", "none")
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_TLS", "none")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV=production and SMTP_TLS=none, got nil")
	}
	if !strings.Contains(err.Error(), "SMTP_TLS") {
		t.Errorf("expected error to mention SMTP_TLS, got: %v", err)
	}
}

// TestLoad_InvalidSMTPTLSMode proves a typo'd SMTP_TLS is a boot-time
// error, not a value silently ignored until the first send attempt.
func TestLoad_InvalidSMTPTLSMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_TLS", "implicit")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for an invalid SMTP_TLS value, got nil")
	}
	if !strings.Contains(err.Error(), "SMTP_TLS") {
		t.Errorf("expected error to mention SMTP_TLS, got: %v", err)
	}
}

// TestLoad_SMTPDefaults proves the SMTP_* defaults (port 587, starttls,
// 10s timeout) match what Mailpit-free development and every provider
// candidate in freeze.md bagian 7 actually need — Mailpit itself
// overrides SMTP_TLS=none via docker-compose.yml, not via this default.
func TestLoad_SMTPDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("expected default SMTPPort=587, got %d", cfg.SMTPPort)
	}
	if cfg.SMTPTLS != "starttls" {
		t.Errorf("expected default SMTPTLS=starttls, got %q", cfg.SMTPTLS)
	}
	if cfg.SMTPTimeout != 10*time.Second {
		t.Errorf("expected default SMTPTimeout=10s, got %s", cfg.SMTPTimeout)
	}
}
