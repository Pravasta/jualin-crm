package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// turnstileVerifyURL is Cloudflare's documented siteverify endpoint —
// never overridden in production; turnstile_internal_test.go points a
// TurnstileVerifier at an httptest.Server standing in for it, the same
// pattern push.FCMSender's own tests use for FCM (there's no local
// Turnstile emulator to run against for real).
const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// TurnstileConfig holds everything TurnstileVerifier needs. Maps
// directly to CAPTCHA_*/TURNSTILE_* env vars (internal/shared/config).
type TurnstileConfig struct {
	SecretKey string
	Timeout   time.Duration
}

// TurnstileVerifier calls Cloudflare's siteverify over HTTP — no SDK
// dependency, same reasoning FCMSender talks to FCM's HTTP v1 API
// directly rather than pulling in a client library for one endpoint
// (Rule #27).
type TurnstileVerifier struct {
	secretKey string
	client    *http.Client
	verifyURL string
}

func NewTurnstileVerifier(cfg TurnstileConfig) *TurnstileVerifier {
	return &TurnstileVerifier{
		secretKey: cfg.SecretKey,
		client:    &http.Client{Timeout: cfg.Timeout},
		verifyURL: turnstileVerifyURL,
	}
}

type turnstileResponse struct {
	Success bool `json:"success"`
}

// Verify never logs v.secretKey or token — Aturan #26 (TURNSTILE_SECRET_KEY
// is a credential exactly like an SMTP password or an FCM service
// account, even though it's only ever sent, never received, by this
// process) and error wrapping below is careful never to interpolate
// either into a returned message.
//
// A token the visitor never sent is rejected without a network call —
// no reason to spend a Cloudflare API round trip confirming what an
// empty string already answers. Any OTHER failure — Cloudflare
// unreachable, a timeout, a response body that doesn't parse, or
// Cloudflare genuinely saying success:false — is treated the same way
// at the HTTP layer above this package (TD §9's single captcha_failed
// code never distinguishes them), but is returned here as a WRAPPED,
// non-ErrCaptchaFailed error for infrastructure problems specifically,
// so a caller that wants to log "our verification pipeline is down"
// separately from "this visitor failed the challenge" still can with
// errors.Is(err, captcha.ErrCaptchaFailed).
func (v *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if token == "" {
		return ErrCaptchaFailed
	}

	form := url.Values{}
	form.Set("secret", v.secretKey)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("captcha: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		// Deliberately fail CLOSED: unlike push/mail (best-effort, never
		// block the triggering action), CAPTCHA is a gate. Cloudflare
		// being unreachable must not silently let every submission
		// through — that would turn one infrastructure hiccup into an
		// open spam window (TD §6: anti-spam here is a cost-control
		// feature, not just security).
		return fmt.Errorf("captcha: verify request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var parsed turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("captcha: decode response: %w", err)
	}
	if !parsed.Success {
		return ErrCaptchaFailed
	}
	return nil
}
