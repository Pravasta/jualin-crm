package config_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
)

// validJWTSecret satisfies the minimum-length requirement so tests that
// aren't specifically exercising JWT_SECRET validation don't fail on it.
const validJWTSecret = "test-jwt-secret-at-least-32-bytes-long"

// validFormTokenSecret is validJWTSecret's Phase 6 counterpart — a
// SEPARATE value on purpose (config.go's own comment: rotating one
// must never force rotating the other).
const validFormTokenSecret = "test-form-token-secret-at-least-32-bytes"

// validWebhookSecretEncKey is the third of these — a separate value for
// the same reason validFormTokenSecret is separate from validJWTSecret.
const validWebhookSecretEncKey = "test-webhook-secret-enc-key-32-bytes-min" // #nosec G101 -- test-only value, not a real credential

// TestMain sets every required-with-no-default env var for every test in
// this file, not just the ones that exercise them directly. Without this,
// every EXISTING test below that doesn't know they exist would fail on
// them before ever reaching whatever it actually means to test —
// env.Parse fails on the FIRST missing required field in struct
// declaration order (verified against caarlos0/env's source), so a test
// focused on, say, CORS validation would otherwise get a
// FORM_TOKEN_SECRET error instead of the one it's asserting on.
// t.Setenv inside an individual test (TestLoad_MissingFormTokenSecret
// etc.) correctly shadows this for that test's own duration only.
func TestMain(m *testing.M) {
	required := map[string]string{
		"FORM_TOKEN_SECRET":      validFormTokenSecret,
		"WEBHOOK_SECRET_ENC_KEY": validWebhookSecretEncKey,
	}
	for k, v := range required {
		if err := os.Setenv(k, v); err != nil {
			fmt.Fprintf(os.Stderr, "failed to set %s: %v\n", k, err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

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
	t.Setenv("PUSH_PROVIDER", "fcm")
	t.Setenv("FCM_PROJECT_ID", "test-project")
	t.Setenv("FCM_CREDENTIALS_FILE", "/tmp/fcm-creds.json")
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("TURNSTILE_SITE_KEY", "test-site-key")
	t.Setenv("TURNSTILE_SECRET_KEY", "test-secret-key")

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

// TestLoad_MissingFormTokenSecret is Phase 6 #87's direct counterpart to
// TestLoad_MissingJWTSecret — FORM_TOKEN_SECRET is required with no
// default, same reasoning: a process booting with a generated or empty
// secret would silently invalidate every time-trap token already
// embedded in a live customer page (TD §6).
func TestLoad_MissingFormTokenSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	unsetEnv(t, "FORM_TOKEN_SECRET")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when FORM_TOKEN_SECRET is missing, got nil")
	}
	if !strings.Contains(err.Error(), "FORM_TOKEN_SECRET") {
		t.Errorf("expected error to mention FORM_TOKEN_SECRET, got: %v", err)
	}
}

func TestLoad_ShortFormTokenSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("FORM_TOKEN_SECRET", "too-short")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for a FORM_TOKEN_SECRET shorter than 32 bytes, got nil")
	}
	if !strings.Contains(err.Error(), "FORM_TOKEN_SECRET") {
		t.Errorf("expected error to mention FORM_TOKEN_SECRET, got: %v", err)
	}
}

// TestLoad_MissingWebhookSecretEncKey is Phase 7 #101's counterpart to
// the two above. Deliberately NOT a production-only check, unlike
// WEBHOOK_ALLOW_PRIVATE_TARGETS: without this key, creating a webhook
// endpoint fails outright in every environment, so there is no
// environment where booting without it is useful (migration 0009).
func TestLoad_MissingWebhookSecretEncKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	unsetEnv(t, "WEBHOOK_SECRET_ENC_KEY")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when WEBHOOK_SECRET_ENC_KEY is missing, got nil")
	}
	if !strings.Contains(err.Error(), "WEBHOOK_SECRET_ENC_KEY") {
		t.Errorf("expected error to mention WEBHOOK_SECRET_ENC_KEY, got: %v", err)
	}
}

func TestLoad_ShortWebhookSecretEncKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("WEBHOOK_SECRET_ENC_KEY", "too-short")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for a WEBHOOK_SECRET_ENC_KEY shorter than 32 bytes, got nil")
	}
	if !strings.Contains(err.Error(), "WEBHOOK_SECRET_ENC_KEY") {
		t.Errorf("expected error to mention WEBHOOK_SECRET_ENC_KEY, got: %v", err)
	}
}

// TestLoad_WebhookSecretEncKeyAccepted proves the happy path actually
// produces a usable key, not just a non-error — the value has to survive
// into Config for crypter.New to receive it.
func TestLoad_WebhookSecretEncKeyAccepted(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("WEBHOOK_SECRET_ENC_KEY", validWebhookSecretEncKey)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WebhookSecretEncKey != validWebhookSecretEncKey {
		t.Errorf("WebhookSecretEncKey = %q, want %q", cfg.WebhookSecretEncKey, validWebhookSecretEncKey)
	}
}

func TestLoad_InvalidCaptchaProvider(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("CAPTCHA_PROVIDER", "recaptcha")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for an unknown CAPTCHA_PROVIDER, got nil")
	}
	if !strings.Contains(err.Error(), "CAPTCHA_PROVIDER") {
		t.Errorf("expected error to mention CAPTCHA_PROVIDER, got: %v", err)
	}
}

// TestLoad_ProductionRejectsNoneCaptchaProvider is issue #87's direct
// acceptance criterion: a production process with anti-spam turned off
// fails completely silently — no crash, no error, just every spam
// submission accepted. Same shape as
// TestLoad_ProductionRejectsLogMailProvider.
func TestLoad_ProductionRejectsNoneCaptchaProvider(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TRUSTED_PROXIES", "none")
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("PUSH_PROVIDER", "fcm")
	t.Setenv("FCM_PROJECT_ID", "test-project")
	t.Setenv("FCM_CREDENTIALS_FILE", "/tmp/fcm-creds.json")
	t.Setenv("CAPTCHA_PROVIDER", "none")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV=production and CAPTCHA_PROVIDER=none, got nil")
	}
	if !strings.Contains(err.Error(), "CAPTCHA_PROVIDER") {
		t.Errorf("expected error to mention CAPTCHA_PROVIDER, got: %v", err)
	}
}

// TestLoad_ProductionRejectsWebhookAllowPrivateTargets is issue #100's
// acceptance criterion: a production process that will POST customer lead
// data to a private or link-local address (SSRF) fails silently — same
// shape as CAPTCHA_PROVIDER=none above.
func TestLoad_ProductionRejectsWebhookAllowPrivateTargets(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TRUSTED_PROXIES", "none")
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("PUSH_PROVIDER", "fcm")
	t.Setenv("FCM_PROJECT_ID", "test-project")
	t.Setenv("FCM_CREDENTIALS_FILE", "/tmp/fcm-creds.json")
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("TURNSTILE_SITE_KEY", "test-site-key")
	t.Setenv("TURNSTILE_SECRET_KEY", "test-secret-key")
	t.Setenv("WEBHOOK_ALLOW_PRIVATE_TARGETS", "true")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV=production and WEBHOOK_ALLOW_PRIVATE_TARGETS=true, got nil")
	}
	if !strings.Contains(err.Error(), "WEBHOOK_ALLOW_PRIVATE_TARGETS") {
		t.Errorf("expected error to mention WEBHOOK_ALLOW_PRIVATE_TARGETS, got: %v", err)
	}
}

func TestLoad_WebhookAllowPrivateTargets_DefaultFalse(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WebhookAllowPrivateTargets {
		t.Error("expected WebhookAllowPrivateTargets to default to false")
	}
}

// TestLoad_ProductionRejectsSubscriptionTestCheckout is Phase 8.5
// #124's acceptance criterion — same shape as
// TestLoad_ProductionRejectsWebhookAllowPrivateTargets above: a
// production process letting every customer upgrade themselves to Pro
// for free fails silently without this guard.
func TestLoad_ProductionRejectsSubscriptionTestCheckout(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TRUSTED_PROXIES", "none")
	t.Setenv("MAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("PUSH_PROVIDER", "fcm")
	t.Setenv("FCM_PROJECT_ID", "test-project")
	t.Setenv("FCM_CREDENTIALS_FILE", "/tmp/fcm-creds.json")
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("TURNSTILE_SITE_KEY", "test-site-key")
	t.Setenv("TURNSTILE_SECRET_KEY", "test-secret-key")
	t.Setenv("SUBSCRIPTION_TEST_CHECKOUT", "true")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV=production and SUBSCRIPTION_TEST_CHECKOUT=true, got nil")
	}
	if !strings.Contains(err.Error(), "SUBSCRIPTION_TEST_CHECKOUT") {
		t.Errorf("expected error to mention SUBSCRIPTION_TEST_CHECKOUT, got: %v", err)
	}
}

func TestLoad_SubscriptionTestCheckout_DefaultFalse(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubscriptionTestCheckout {
		t.Error("expected SubscriptionTestCheckout to default to false")
	}
}

// TestLoad_SubscriptionAdminToken_EmptyIsValid proves empty is a
// legitimate value (route disabled), not a misconfiguration — unlike
// WebhookSecretEncKey, this one has no `required` tag.
func TestLoad_SubscriptionAdminToken_EmptyIsValid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SubscriptionAdminToken != "" {
		t.Errorf("expected SubscriptionAdminToken to default to empty, got %q", cfg.SubscriptionAdminToken)
	}
}

func TestLoad_SubscriptionAdminToken_TooShort_Rejected(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("SUBSCRIPTION_ADMIN_TOKEN", "too-short")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for a SUBSCRIPTION_ADMIN_TOKEN shorter than the minimum, got nil")
	}
	if !strings.Contains(err.Error(), "SUBSCRIPTION_ADMIN_TOKEN") {
		t.Errorf("expected error to mention SUBSCRIPTION_ADMIN_TOKEN, got: %v", err)
	}
}

func TestLoad_SubscriptionAdminToken_LongEnough_Accepted(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("SUBSCRIPTION_ADMIN_TOKEN", strings.Repeat("a", 32))

	if _, err := config.Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_TurnstileRequiresSiteKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("TURNSTILE_SECRET_KEY", "test-secret-key")
	unsetEnv(t, "TURNSTILE_SITE_KEY")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when CAPTCHA_PROVIDER=turnstile and TURNSTILE_SITE_KEY is unset, got nil")
	}
	if !strings.Contains(err.Error(), "TURNSTILE_SITE_KEY") {
		t.Errorf("expected error to mention TURNSTILE_SITE_KEY, got: %v", err)
	}
}

func TestLoad_TurnstileRequiresSecretKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("TURNSTILE_SITE_KEY", "test-site-key")
	unsetEnv(t, "TURNSTILE_SECRET_KEY")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when CAPTCHA_PROVIDER=turnstile and TURNSTILE_SECRET_KEY is unset, got nil")
	}
	if !strings.Contains(err.Error(), "TURNSTILE_SECRET_KEY") {
		t.Errorf("expected error to mention TURNSTILE_SECRET_KEY, got: %v", err)
	}
}

func TestLoad_CaptchaNoneIsValidOutsideProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CaptchaProvider != "none" {
		t.Errorf(`expected default CaptchaProvider "none", got %q`, cfg.CaptchaProvider)
	}
}

func TestLoad_FormSubmitRateLimitIP_MustBePositive(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("FORM_SUBMIT_RATE_LIMIT_IP", "0")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for FORM_SUBMIT_RATE_LIMIT_IP=0, got nil")
	}
	if !strings.Contains(err.Error(), "FORM_SUBMIT_RATE_LIMIT_IP") {
		t.Errorf("expected error to mention FORM_SUBMIT_RATE_LIMIT_IP, got: %v", err)
	}
}

func TestLoad_FormSubmitRateLimitForm_MustBePositive(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("FORM_SUBMIT_RATE_LIMIT_FORM", "-1")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for FORM_SUBMIT_RATE_LIMIT_FORM=-1, got nil")
	}
	if !strings.Contains(err.Error(), "FORM_SUBMIT_RATE_LIMIT_FORM") {
		t.Errorf("expected error to mention FORM_SUBMIT_RATE_LIMIT_FORM, got: %v", err)
	}
}

func TestLoad_FormSubmitRateLimitDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.FormSubmitRateLimitIP != 20 {
		t.Errorf("expected default FormSubmitRateLimitIP=20, got %d", cfg.FormSubmitRateLimitIP)
	}
	if cfg.FormSubmitRateLimitForm != 60 {
		t.Errorf("expected default FormSubmitRateLimitForm=60, got %d", cfg.FormSubmitRateLimitForm)
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
	t.Setenv("PUSH_PROVIDER", "fcm")
	t.Setenv("FCM_PROJECT_ID", "test-project")
	t.Setenv("FCM_CREDENTIALS_FILE", "/tmp/fcm-creds.json")
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("TURNSTILE_SITE_KEY", "test-site-key")
	t.Setenv("TURNSTILE_SECRET_KEY", "test-secret-key")

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
	t.Setenv("PUSH_PROVIDER", "fcm")
	t.Setenv("FCM_PROJECT_ID", "test-project")
	t.Setenv("FCM_CREDENTIALS_FILE", "/tmp/fcm-creds.json")
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("TURNSTILE_SITE_KEY", "test-site-key")
	t.Setenv("TURNSTILE_SECRET_KEY", "test-secret-key")
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
	t.Setenv("PUSH_PROVIDER", "fcm")
	t.Setenv("FCM_PROJECT_ID", "test-project")
	t.Setenv("FCM_CREDENTIALS_FILE", "/tmp/fcm-creds.json")
	t.Setenv("CAPTCHA_PROVIDER", "turnstile")
	t.Setenv("TURNSTILE_SITE_KEY", "test-site-key")
	t.Setenv("TURNSTILE_SECRET_KEY", "test-secret-key")
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

// The webhook worker knobs (Phase 7 #102, TD §9). Validated even when
// WEBHOOK_WORKER_ENABLED is false: a nonsense value should fail at the
// boot that introduced it, not at the later one that turns the worker
// back on.
func TestLoad_WebhookWorkerDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.WebhookWorkerEnabled {
		t.Error("expected the worker to be enabled by default")
	}
	if cfg.WebhookWorkerInterval != 10*time.Second {
		t.Errorf("WebhookWorkerInterval = %s, want 10s", cfg.WebhookWorkerInterval)
	}
	if cfg.WebhookWorkerBatch != 20 {
		t.Errorf("WebhookWorkerBatch = %d, want 20", cfg.WebhookWorkerBatch)
	}
	if cfg.WebhookDeliveryTimeout != 10*time.Second {
		t.Errorf("WebhookDeliveryTimeout = %s, want 10s", cfg.WebhookDeliveryTimeout)
	}
	if cfg.WebhookMaxAttempts != 5 {
		t.Errorf("WebhookMaxAttempts = %d, want 5", cfg.WebhookMaxAttempts)
	}
	if cfg.WebhookDeliveryRetentionDays != 30 {
		t.Errorf("WebhookDeliveryRetentionDays = %d, want 30", cfg.WebhookDeliveryRetentionDays)
	}
}

func TestLoad_WebhookWorkerRejectsNonsenseValues(t *testing.T) {
	for _, tc := range []struct{ key, value, want string }{
		{"WEBHOOK_WORKER_INTERVAL", "0s", "WEBHOOK_WORKER_INTERVAL"},
		{"WEBHOOK_WORKER_INTERVAL", "-5s", "WEBHOOK_WORKER_INTERVAL"},
		{"WEBHOOK_WORKER_BATCH", "0", "WEBHOOK_WORKER_BATCH"},
		{"WEBHOOK_DELIVERY_TIMEOUT", "0s", "WEBHOOK_DELIVERY_TIMEOUT"},
		{"WEBHOOK_MAX_ATTEMPTS", "0", "WEBHOOK_MAX_ATTEMPTS"},
		{"WEBHOOK_DELIVERY_RETENTION_DAYS", "0", "WEBHOOK_DELIVERY_RETENTION_DAYS"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv("JWT_SECRET", validJWTSecret)
			t.Setenv(tc.key, tc.value)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected %s=%s to be rejected", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected the error to name %s, got: %v", tc.want, err)
			}
		})
	}
}

// TestLoad_WebhookWorkerKnobsValidatedEvenWhenDisabled is the reason the
// checks sit outside any `if enabled` branch.
func TestLoad_WebhookWorkerKnobsValidatedEvenWhenDisabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)
	t.Setenv("WEBHOOK_WORKER_ENABLED", "false")
	t.Setenv("WEBHOOK_WORKER_BATCH", "0")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected an invalid batch size to be rejected even with the worker disabled")
	}
}

// --- ENTERPRISE_CONTACT_URL (Phase 8.5 follow-up) ---

// TestLoad_EnterpriseContactURL_EmptyIsValid locks the "no destination
// yet" state as legitimate: the Langganan screen renders the Enterprise
// card as plain text rather than a dead link.
func TestLoad_EnterpriseContactURL_EmptyIsValid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", validJWTSecret)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected an empty ENTERPRISE_CONTACT_URL to be valid, got: %v", err)
	}
	if cfg.EnterpriseContactURL != "" {
		t.Errorf("expected empty by default, got %q", cfg.EnterpriseContactURL)
	}
}

func TestLoad_EnterpriseContactURL_AcceptedSchemes(t *testing.T) {
	for _, raw := range []string{
		"https://wa.me/6281234567890",
		"https://wa.me/6281234567890?text=Halo",
		"mailto:halo@jualin.example",
	} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv("JWT_SECRET", validJWTSecret)
			t.Setenv("ENTERPRISE_CONTACT_URL", raw)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("expected %q to be accepted, got: %v", raw, err)
			}
			if cfg.EnterpriseContactURL != raw {
				t.Errorf("got %q, want %q", cfg.EnterpriseContactURL, raw)
			}
		})
	}
}

// TestLoad_EnterpriseContactURL_RejectedSchemes is the security half of
// this value: it lands in an href on the Langganan screen, so a
// javascript: URL would execute in the viewer's session. Rejected at
// boot rather than trusted to be escaped correctly forever downstream.
func TestLoad_EnterpriseContactURL_RejectedSchemes(t *testing.T) {
	for _, raw := range []string{
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"http://wa.me/6281234567890", // plaintext downgrade from an HTTPS dashboard
		"6281234567890",              // a bare phone number is not a URL
		"wa.me/6281234567890",        // no scheme at all
	} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv("JWT_SECRET", validJWTSecret)
			t.Setenv("ENTERPRISE_CONTACT_URL", raw)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected %q to be rejected — it is rendered as a link", raw)
			}
			if !strings.Contains(err.Error(), "ENTERPRISE_CONTACT_URL") {
				t.Errorf("expected the error to name ENTERPRISE_CONTACT_URL, got: %v", err)
			}
		})
	}
}
