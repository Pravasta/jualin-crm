// Package captcha defines the interface every form-submission flow in
// this codebase verifies anti-bot challenges through, and a
// NoopVerifier for development and test — same shape as
// internal/shared/mailer (Phase 4.6) and internal/shared/push (Phase
// 5): an interface with one real implementation would normally violate
// Rule #27, justified here because the second implementation
// (TurnstileVerifier) is built in the same issue (Phase 6 #87, TD §6
// keputusan D2), not deferred.
package captcha

import "context"

// ErrCaptchaFailed is the ONE error a Verifier ever returns for a
// rejected or absent token — TD §9's single error code (captcha_failed)
// collapses "Cloudflare said no", "no token was sent at all", and
// "verification itself couldn't be reached" into one outcome. A public,
// unauthenticated caller gains nothing from knowing which — only
// whether the submission is allowed to proceed.
var ErrCaptchaFailed = errCaptchaFailed{}

type errCaptchaFailed struct{}

func (errCaptchaFailed) Error() string { return "captcha: verification failed" }

// Verifier checks a CAPTCHA response token. remoteIP is the submitter's
// address, passed through to the provider when it has one to give
// (Turnstile's own siteverify accepts it optionally) — never required,
// since NoopVerifier ignores both arguments entirely.
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

// NoopVerifier always succeeds, regardless of token — CAPTCHA_PROVIDER=none
// (TD §6: "cukup untuk seluruh pengembangan"). Rejected outright at boot
// when APP_ENV=production (internal/shared/config), same reasoning
// mailer.LogMailer and push.NoopSender are.
type NoopVerifier struct{}

func (NoopVerifier) Verify(_ context.Context, _, _ string) error { return nil }
