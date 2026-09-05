// Package config loads and validates application configuration from
// environment variables. Config is loaded once at boot; invalid config
// fails startup immediately with a message naming the offending variable.
package config

import (
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/crypter"
)

var validAppEnvs = []string{"development", "production"}
var validLogLevels = []string{"debug", "info", "warn", "error"}

// minJWTSecretLength matches TD phase 1 §2 — enough entropy that HS256
// signatures aren't brute-forceable.
const minJWTSecretLength = 32

// "smtp" added in Phase 4.6 (issue #63) — mailer.Mailer's second real
// implementation, the one its own Phase 1 comment already said was
// coming (Rule #27).
var validMailProviders = []string{"log", "smtp"}

var validSMTPTLSModes = []string{"starttls", "none"}

// "none" (push.NoopSender) or "fcm" (push.FCMSender) — same shape as
// validMailProviders (Phase 5 #68).
var validPushProviders = []string{"none", "fcm"}

// "none" (captcha.NoopVerifier, always passes) or "turnstile"
// (captcha.TurnstileVerifier) — same shape as validMailProviders/
// validPushProviders (Phase 6 #87, TD §6 keputusan D2).
var validCaptchaProviders = []string{"none", "turnstile"}

// minFormTokenSecretLength mirrors minJWTSecretLength (Aturan #36) —
// FORM_TOKEN_SECRET is its own value, never JWT_SECRET reused, so
// rotating one never forces rotating the other (TD §6).
const minFormTokenSecretLength = 32

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

	// SMTP* configure mailer.SMTPMailer (Phase 4.6, issue #63) — read only
	// when MailProvider is "smtp". Username/Password may both be empty
	// (Mailpit in development takes no auth); AUTH is skipped entirely in
	// that case, not attempted with empty credentials.
	SMTPHost     string        `env:"SMTP_HOST"`
	SMTPPort     int           `env:"SMTP_PORT" envDefault:"587"`
	SMTPUsername string        `env:"SMTP_USERNAME"`
	SMTPPassword string        `env:"SMTP_PASSWORD"`
	SMTPTLS      string        `env:"SMTP_TLS" envDefault:"starttls"`
	SMTPTimeout  time.Duration `env:"SMTP_TIMEOUT" envDefault:"10s"`

	// Push* configure push.FCMSender (Phase 5, issue #68) — read only
	// when PushProvider is "fcm". Same shape as MailProvider/SMTP*
	// above, deliberately: "none" (push.NoopSender) is rejected outright
	// when APP_ENV=production, for the identical reason MAIL_PROVIDER=log
	// is — a production process that silently never pushes fails with no
	// error at all.
	PushProvider       string        `env:"PUSH_PROVIDER" envDefault:"none"`
	FCMProjectID       string        `env:"FCM_PROJECT_ID"`
	FCMCredentialsFile string        `env:"FCM_CREDENTIALS_FILE"`
	PushTimeout        time.Duration `env:"PUSH_TIMEOUT" envDefault:"10s"`

	// JWTSecret signs and verifies access tokens (HS256, TD phase 1 §4).
	// Required with no default — an app booting with a generated or empty
	// secret would silently invalidate every token on the next restart.
	JWTSecret                string        `env:"JWT_SECRET,required"`
	AccessTokenTTL           time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTLDashboard time.Duration `env:"REFRESH_TOKEN_TTL_DASHBOARD" envDefault:"720h"`
	RefreshTokenTTLMobile    time.Duration `env:"REFRESH_TOKEN_TTL_MOBILE" envDefault:"2160h"`

	// CookieDomain empty = host-only cookie, correct for localhost.
	CookieDomain string `env:"COOKIE_DOMAIN" envDefault:""`
	// CookieSecure must be true whenever AppEnv is production — validated
	// below, not just documented, because COOKIE_SECURE=false in
	// production means auth cookies travel over plain HTTP (Rule #36).
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"false"`

	// CORSAllowedOrigins are the browser origins allowed to call this API
	// with credentials (Phase 3 TD §1.1) — never "*": Access-Control-
	// Allow-Credentials: true and a wildcard origin are mutually
	// exclusive, so a list is the only option once cookies are involved.
	// Empty by default — local development writes
	// http://localhost:3000 explicitly in .env/docker-compose.yml rather
	// than having it hidden as a code default; required non-empty in
	// production below (Rule #36).
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`

	// PublicAPIRateLimit is POST /v1/leads' per-api_key request budget
	// (Phase 4 TD §6, keputusan D4) — requests per minute, fixed window.
	// Not a measured number: it's two orders of magnitude above normal
	// SMB traffic, exposed via env so it can be tuned once real
	// integrators exist without a deploy.
	PublicAPIRateLimit int `env:"PUBLIC_API_RATE_LIMIT" envDefault:"60"`

	// FormTokenSecret signs the time-trap token embedded in the embed
	// page (Phase 6 #87, TD §6) — a SEPARATE secret from JWTSecret on
	// purpose, so rotating one never forces rotating the other. Required
	// with no default, same reasoning as JWTSecret: a process booting
	// with a generated or empty secret would silently invalidate every
	// token already embedded in a live customer page.
	FormTokenSecret string `env:"FORM_TOKEN_SECRET,required"`

	// Captcha* configure captcha.Verifier (Phase 6 #87, TD §6 keputusan
	// D2) — same shape as MailProvider/PushProvider above. "none"
	// (captcha.NoopVerifier, always passes) is enough for the whole
	// phase except real anti-spam verification itself; rejected outright
	// when APP_ENV=production for the identical silent-failure reason
	// MAIL_PROVIDER=log and PUSH_PROVIDER=none are.
	CaptchaProvider    string        `env:"CAPTCHA_PROVIDER" envDefault:"none"`
	TurnstileSiteKey   string        `env:"TURNSTILE_SITE_KEY"`
	TurnstileSecretKey string        `env:"TURNSTILE_SECRET_KEY"`
	CaptchaTimeout     time.Duration `env:"CAPTCHA_TIMEOUT" envDefault:"5s"`

	// FormSubmitRateLimit{IP,Form} are POST /v1/forms/{public_key}/submit's
	// two independent budgets (Phase 6 #87, TD §6 keputusan D4) —
	// requests per minute, fixed window, one bucket keyed by IP and one
	// by public_key. Not measured numbers, same honesty as
	// PublicAPIRateLimit's own comment.
	FormSubmitRateLimitIP   int `env:"FORM_SUBMIT_RATE_LIMIT_IP" envDefault:"20"`
	FormSubmitRateLimitForm int `env:"FORM_SUBMIT_RATE_LIMIT_FORM" envDefault:"60"`

	// WebhookAllowPrivateTargets relaxes safedial's SSRF guard so
	// outbound webhooks can reach private/loopback/link-local addresses
	// (Phase 7 #100, TD §3.4) — needed only for local development
	// (http://localhost:9099 as a test receiver). Rejected outright when
	// APP_ENV=production, same shape and same silent-failure reasoning as
	// CAPTCHA_PROVIDER=none / PUSH_PROVIDER=none / MAIL_PROVIDER=log: a
	// production process that will happily POST a customer's lead data to
	// 169.254.169.254 is not a degraded mode, it's a hole.
	WebhookAllowPrivateTargets bool `env:"WEBHOOK_ALLOW_PRIVATE_TARGETS" envDefault:"false"`

	// WebhookSecretEncKey is the AES-256-GCM key protecting webhook
	// signing secrets at rest (Phase 7 #101, migration 0009). A THIRD
	// independent secret alongside JWTSecret and FormTokenSecret, for the
	// same reason those two are separate from each other: rotating one
	// must never force rotating the others, and these three protect
	// unrelated things with unrelated lifetimes.
	//
	// Required with no default, and unlike WebhookAllowPrivateTargets
	// this is NOT a production-only check — an empty key doesn't degrade
	// webhooks, it makes creating an endpoint fail outright, so there is
	// no environment where booting without it is useful. Rotating it
	// makes every stored secret undecryptable; that's a documented
	// consequence, not a bug (td.md §2).
	WebhookSecretEncKey string `env:"WEBHOOK_SECRET_ENC_KEY,required"`

	// Webhook worker knobs (Phase 7 #102, TD §9). The worker is a
	// goroutine inside this binary, not a separate deployable — D2.
	//
	// WebhookWorkerEnabled false leaves the HTTP API fully functional and
	// deliveries accumulating as pending: correct for an instance that
	// should only serve traffic. It is NOT a production guard like
	// WEBHOOK_ALLOW_PRIVATE_TARGETS — a fleet where every instance sets it
	// false is a deployment mistake this process cannot detect alone.
	//
	// Interval, batch, and MaxAttempts are NOT measured numbers; they join
	// the shared review in api.md's "Angka batasnya belum pernah diukur"
	// (issue #98), not a separate open question each.
	WebhookWorkerEnabled   bool          `env:"WEBHOOK_WORKER_ENABLED" envDefault:"true"`
	WebhookWorkerInterval  time.Duration `env:"WEBHOOK_WORKER_INTERVAL" envDefault:"10s"`
	WebhookWorkerBatch     int           `env:"WEBHOOK_WORKER_BATCH" envDefault:"20"`
	WebhookDeliveryTimeout time.Duration `env:"WEBHOOK_DELIVERY_TIMEOUT" envDefault:"10s"`

	// WebhookMaxAttempts is the number of RETRIES after the initial
	// delivery, not the total number of sends — the default 5 means up to
	// 6 HTTP calls, spaced by the five delays in webhook.retryDelays
	// (1m, 5m, 30m, 2h, 6h). Setting it above 5 is allowed and defined:
	// every retry past the table reuses the last delay (6h). Not bounded
	// against that table here on purpose — internal/shared/config must not
	// import a domain package to learn its length (ADR-011).
	WebhookMaxAttempts int `env:"WEBHOOK_MAX_ATTEMPTS" envDefault:"5"`

	WebhookDeliveryRetentionDays int `env:"WEBHOOK_DELIVERY_RETENTION_DAYS" envDefault:"30"`

	// TrustedProxies lists the IPs/CIDRs whose X-Forwarded-For/X-Real-IP
	// header may be believed (Phase 4.5 TD §1). The literal "none" means
	// no reverse proxy sits in front of this process, so the connection's
	// own peer address is used directly. Required when AppEnv is
	// production (Rule #36) — both wrong settings fail silently in
	// opposite directions: trusting everyone (Gin's own default) makes
	// every per-IP rate limit forgeable with one header (Rule #34);
	// trusting nobody while actually behind a load balancer collapses
	// every real client onto one IP and one shared limiter bucket.
	TrustedProxies []string `env:"TRUSTED_PROXIES" envSeparator:","`

	// SubscriptionAdminToken gates POST /internal/subscriptions/{id}/plan
	// (Phase 8.5 #124) — the first surface in this product authenticated
	// by a bearer token rather than as ANY principal (user/api_key/
	// public_form). Empty means the route is not registered at all: a
	// deployment that never sets it gets a 404, not a route that can
	// never succeed — the same "don't expose a door with no working key"
	// reasoning as WEBHOOK_SECRET_ENC_KEY being required rather than
	// silently degrading. Compared with subtle.ConstantTimeCompare,
	// never logged (Rule #26), never sent to any client (Rule #23).
	SubscriptionAdminToken string `env:"SUBSCRIPTION_ADMIN_TOKEN" envDefault:""`

	// SubscriptionTestCheckout gates POST /v1/subscription/test-checkout
	// (Phase 8.5 #124) — an Owner-triggered upgrade to Pro with no real
	// payment behind it, for exercising the paid-plan gates before
	// payment service integration lands (prd D6). false means the route
	// is not registered: same "not just hidden, not present" shape as
	// SubscriptionAdminToken above. Rejected outright in production —
	// letting every customer upgrade themselves for free is not a
	// degraded mode, it's a hole (same reasoning as
	// WEBHOOK_ALLOW_PRIVATE_TARGETS).
	SubscriptionTestCheckout bool `env:"SUBSCRIPTION_TEST_CHECKOUT" envDefault:"false"`
}

// minSubscriptionAdminTokenLength mirrors WebhookSecretEncKey's own
// minimum — long enough that guessing it is not a realistic attack,
// short enough that generating one is a one-line `openssl rand -hex 32`.
const minSubscriptionAdminTokenLength = 32

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
	// A production process that never sends email fails silently — no
	// error, no crash, just a registration funnel that quietly stops
	// working (Phase 4.6 decision E2). LogMailer also writes the
	// verification token itself to the log, a single-use credential
	// (Rule #26) — reason enough on its own.
	if c.AppEnv == "production" && c.MailProvider == "log" {
		return fmt.Errorf(`config invalid: MAIL_PROVIDER must not be "log" when APP_ENV=production`)
	}
	if c.MailProvider == "smtp" {
		if c.SMTPHost == "" {
			return fmt.Errorf("config invalid: SMTP_HOST must be set when MAIL_PROVIDER=smtp")
		}
		if c.SMTPPort <= 0 || c.SMTPPort > 65535 {
			return fmt.Errorf("config invalid: SMTP_PORT must be between 1 and 65535, got %d", c.SMTPPort)
		}
		if !slices.Contains(validSMTPTLSModes, c.SMTPTLS) {
			return fmt.Errorf("config invalid: SMTP_TLS must be one of %v, got %q", validSMTPTLSModes, c.SMTPTLS)
		}
		if c.AppEnv == "production" && c.SMTPTLS == "none" {
			return fmt.Errorf("config invalid: SMTP_TLS must not be \"none\" when APP_ENV=production")
		}
		if c.SMTPTimeout <= 0 {
			return fmt.Errorf("config invalid: SMTP_TIMEOUT must be positive, got %s", c.SMTPTimeout)
		}
	}
	if !slices.Contains(validPushProviders, c.PushProvider) {
		return fmt.Errorf("config invalid: PUSH_PROVIDER must be one of %v, got %q", validPushProviders, c.PushProvider)
	}
	// Same reasoning as MAIL_PROVIDER=log in production (above): a
	// process that never pushes fails with no error, no crash — just a
	// notification that quietly never arrives on anyone's phone.
	if c.AppEnv == "production" && c.PushProvider == "none" {
		return fmt.Errorf(`config invalid: PUSH_PROVIDER must not be "none" when APP_ENV=production`)
	}
	if c.PushProvider == "fcm" {
		if c.FCMProjectID == "" {
			return fmt.Errorf("config invalid: FCM_PROJECT_ID must be set when PUSH_PROVIDER=fcm")
		}
		if c.FCMCredentialsFile == "" {
			return fmt.Errorf("config invalid: FCM_CREDENTIALS_FILE must be set when PUSH_PROVIDER=fcm")
		}
		if c.PushTimeout <= 0 {
			return fmt.Errorf("config invalid: PUSH_TIMEOUT must be positive, got %s", c.PushTimeout)
		}
	}
	if len(c.JWTSecret) < minJWTSecretLength {
		return fmt.Errorf("config invalid: JWT_SECRET must be at least %d bytes, got %d", minJWTSecretLength, len(c.JWTSecret))
	}
	if len(c.FormTokenSecret) < minFormTokenSecretLength {
		return fmt.Errorf("config invalid: FORM_TOKEN_SECRET must be at least %d bytes, got %d", minFormTokenSecretLength, len(c.FormTokenSecret))
	}
	// crypter.MinKeyLength rather than a fourth local constant — the
	// floor belongs to the thing that consumes the key, and duplicating
	// it here is how the two drift apart later.
	if len(c.WebhookSecretEncKey) < crypter.MinKeyLength {
		return fmt.Errorf("config invalid: WEBHOOK_SECRET_ENC_KEY must be at least %d bytes, got %d", crypter.MinKeyLength, len(c.WebhookSecretEncKey))
	}
	if !slices.Contains(validCaptchaProviders, c.CaptchaProvider) {
		return fmt.Errorf("config invalid: CAPTCHA_PROVIDER must be one of %v, got %q", validCaptchaProviders, c.CaptchaProvider)
	}
	// A production process with anti-spam turned off fails silently —
	// no error, no crash, just every spam submission accepted (Phase 6
	// TD §6). Identical reasoning to MAIL_PROVIDER=log and
	// PUSH_PROVIDER=none being rejected in production above.
	if c.AppEnv == "production" && c.CaptchaProvider == "none" {
		return fmt.Errorf(`config invalid: CAPTCHA_PROVIDER must not be "none" when APP_ENV=production`)
	}
	if c.CaptchaProvider == "turnstile" {
		if c.TurnstileSiteKey == "" {
			return fmt.Errorf("config invalid: TURNSTILE_SITE_KEY must be set when CAPTCHA_PROVIDER=turnstile")
		}
		if c.TurnstileSecretKey == "" {
			return fmt.Errorf("config invalid: TURNSTILE_SECRET_KEY must be set when CAPTCHA_PROVIDER=turnstile")
		}
		if c.CaptchaTimeout <= 0 {
			return fmt.Errorf("config invalid: CAPTCHA_TIMEOUT must be positive, got %s", c.CaptchaTimeout)
		}
	}
	if c.FormSubmitRateLimitIP <= 0 {
		return fmt.Errorf("config invalid: FORM_SUBMIT_RATE_LIMIT_IP must be positive, got %d", c.FormSubmitRateLimitIP)
	}
	if c.FormSubmitRateLimitForm <= 0 {
		return fmt.Errorf("config invalid: FORM_SUBMIT_RATE_LIMIT_FORM must be positive, got %d", c.FormSubmitRateLimitForm)
	}
	// A production process that will POST customer lead data to a private
	// or link-local address fails silently — no error, just SSRF (Phase 7
	// TD §3.4). Same shape as CAPTCHA_PROVIDER=none above.
	if c.AppEnv == "production" && c.WebhookAllowPrivateTargets {
		return fmt.Errorf(`config invalid: WEBHOOK_ALLOW_PRIVATE_TARGETS must not be true when APP_ENV=production`)
	}
	// Worker knobs are only read when the worker runs, but they are
	// validated unconditionally: a value that would be nonsense on the day
	// someone flips WEBHOOK_WORKER_ENABLED back on should fail at the boot
	// that introduced it, not at that later one (Rule #36).
	if c.WebhookWorkerInterval <= 0 {
		return fmt.Errorf("config invalid: WEBHOOK_WORKER_INTERVAL must be positive, got %s", c.WebhookWorkerInterval)
	}
	if c.WebhookWorkerBatch < 1 {
		return fmt.Errorf("config invalid: WEBHOOK_WORKER_BATCH must be at least 1, got %d", c.WebhookWorkerBatch)
	}
	if c.WebhookDeliveryTimeout <= 0 {
		return fmt.Errorf("config invalid: WEBHOOK_DELIVERY_TIMEOUT must be positive, got %s", c.WebhookDeliveryTimeout)
	}
	if c.WebhookMaxAttempts < 1 {
		return fmt.Errorf("config invalid: WEBHOOK_MAX_ATTEMPTS must be at least 1, got %d", c.WebhookMaxAttempts)
	}
	if c.WebhookDeliveryRetentionDays < 1 {
		return fmt.Errorf("config invalid: WEBHOOK_DELIVERY_RETENTION_DAYS must be at least 1, got %d", c.WebhookDeliveryRetentionDays)
	}
	if c.AppEnv == "production" && !c.CookieSecure {
		return fmt.Errorf("config invalid: COOKIE_SECURE must be true when APP_ENV=production")
	}
	if c.AppEnv == "production" && len(c.CORSAllowedOrigins) == 0 {
		return fmt.Errorf("config invalid: CORS_ALLOWED_ORIGINS must be set when APP_ENV=production")
	}
	if c.PublicAPIRateLimit <= 0 {
		return fmt.Errorf("config invalid: PUBLIC_API_RATE_LIMIT must be positive, got %d", c.PublicAPIRateLimit)
	}
	if c.AppEnv == "production" && len(c.TrustedProxies) == 0 {
		return fmt.Errorf(`config invalid: TRUSTED_PROXIES must be set when APP_ENV=production (use "none" when no reverse proxy sits in front of this process)`)
	}
	if len(c.TrustedProxies) > 1 && slices.Contains(c.TrustedProxies, "none") {
		return fmt.Errorf(`config invalid: TRUSTED_PROXIES cannot mix "none" with actual proxy entries`)
	}
	for _, p := range c.TrustedProxies {
		if p == "none" {
			continue
		}
		if !validProxyEntry(p) {
			return fmt.Errorf("config invalid: TRUSTED_PROXIES entry %q is not a valid IP or CIDR", p)
		}
	}
	// A short token is a token an attacker can brute-force; a set-but-weak
	// token is worse than the route not existing at all (empty — see the
	// field's own doc comment). Only checked when SET: empty is the
	// legitimate "route disabled" state, not a misconfiguration.
	if c.SubscriptionAdminToken != "" && len(c.SubscriptionAdminToken) < minSubscriptionAdminTokenLength {
		return fmt.Errorf("config invalid: SUBSCRIPTION_ADMIN_TOKEN must be at least %d bytes if set, got %d", minSubscriptionAdminTokenLength, len(c.SubscriptionAdminToken))
	}
	// A production process letting every customer upgrade themselves to
	// Pro for free fails silently — no error, just free Pro accounts
	// (Phase 8.5 TD §8). Same shape as CAPTCHA_PROVIDER=none and
	// WEBHOOK_ALLOW_PRIVATE_TARGETS above.
	if c.AppEnv == "production" && c.SubscriptionTestCheckout {
		return fmt.Errorf(`config invalid: SUBSCRIPTION_TEST_CHECKOUT must not be true when APP_ENV=production`)
	}
	return nil
}

// validProxyEntry mirrors what gin.Engine.SetTrustedProxies itself accepts
// (gin.go's prepareTrustedCIDRs) — a bare IP or a CIDR block — so a value
// that passes validation here is guaranteed to be accepted there too.
// Rejecting it at boot means a typo is a startup error, not a proxy that
// silently never gets trusted.
func validProxyEntry(entry string) bool {
	if net.ParseIP(entry) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(entry)
	return err == nil
}

// IsProduction reports whether the app is running in production mode.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// TrustedProxyCIDRs returns TrustedProxies in the form
// (*gin.Engine).SetTrustedProxies expects. Empty or ["none"] returns nil,
// which is gin's own signal to trust no peer at all — ClientIP() then
// returns the connection's real remote address instead of consulting any
// header (see gin.Engine.isTrustedProxy).
func (c *Config) TrustedProxyCIDRs() []string {
	if len(c.TrustedProxies) == 0 {
		return nil
	}
	if len(c.TrustedProxies) == 1 && c.TrustedProxies[0] == "none" {
		return nil
	}
	return c.TrustedProxies
}
