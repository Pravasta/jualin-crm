package captcha

// package captcha (not captcha_test) — TurnstileVerifier's verifyURL
// field is unexported by design (only ever overridden from here,
// pointed at an httptest.Server standing in for Cloudflare — same
// reasoning push.FCMSender's own fcm_internal_test.go documents: there's
// no local Turnstile emulator to test against for real).

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestVerifier(serverURL string) *TurnstileVerifier {
	return &TurnstileVerifier{
		secretKey: "test-secret-key",
		client:    &http.Client{Timeout: 5 * time.Second},
		verifyURL: serverURL,
	}
}

func TestTurnstileVerifier_Verify_SendsCorrectRequestShape(t *testing.T) {
	var gotMethod, gotContentType string
	var gotForm url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		_ = r.ParseForm()
		gotForm = r.PostForm
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	v := &TurnstileVerifier{secretKey: "my-secret", client: &http.Client{Timeout: 5 * time.Second}, verifyURL: srv.URL}
	err := v.Verify(context.Background(), "response-token-abc", "203.0.113.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("expected form-encoded content type, got %q", gotContentType)
	}
	if gotForm.Get("secret") != "my-secret" {
		t.Errorf("expected secret=my-secret, got %q", gotForm.Get("secret"))
	}
	if gotForm.Get("response") != "response-token-abc" {
		t.Errorf("expected response=response-token-abc, got %q", gotForm.Get("response"))
	}
	if gotForm.Get("remoteip") != "203.0.113.5" {
		t.Errorf("expected remoteip=203.0.113.5, got %q", gotForm.Get("remoteip"))
	}
}

func TestTurnstileVerifier_Verify_EmptyToken_RejectsWithoutNetworkCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	v := newTestVerifier(srv.URL)
	err := v.Verify(context.Background(), "", "203.0.113.5")
	if !errors.Is(err, ErrCaptchaFailed) {
		t.Errorf("expected ErrCaptchaFailed for an empty token, got: %v", err)
	}
	if called {
		t.Error("expected an empty token to never reach the network — no reason to spend a Cloudflare round trip on it")
	}
}

func TestTurnstileVerifier_Verify_CloudflareRejects_ReturnsErrCaptchaFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	defer srv.Close()

	v := newTestVerifier(srv.URL)
	err := v.Verify(context.Background(), "bad-token", "")
	if !errors.Is(err, ErrCaptchaFailed) {
		t.Errorf("expected ErrCaptchaFailed for success:false, got: %v", err)
	}
}

// TestTurnstileVerifier_Verify_Unreachable_FailsClosed proves a network
// failure talking to Cloudflare produces an error (submission rejected,
// D2's deliberate fail-closed choice) rather than nil (submission
// silently let through). Deliberately NOT errors.Is(err, ErrCaptchaFailed)
// — see Verify's own doc comment on why infra failures are a distinct,
// wrapped error from a genuinely-rejected token.
func TestTurnstileVerifier_Verify_Unreachable_FailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // closed before any request — guarantees connection refused

	v := newTestVerifier(srv.URL)
	err := v.Verify(context.Background(), "some-token", "")
	if err == nil {
		t.Fatal("expected an error when Cloudflare is unreachable — fail closed, never silently pass")
	}
}

// TestTurnstileVerifier_Verify_MalformedResponseBody_FailsClosed proves
// a response that isn't valid JSON also fails closed — the same
// reasoning as the unreachable case, just a different failure shape.
func TestTurnstileVerifier_Verify_MalformedResponseBody_FailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	v := newTestVerifier(srv.URL)
	err := v.Verify(context.Background(), "some-token", "")
	if err == nil {
		t.Fatal("expected an error for a malformed response body — fail closed, never silently pass")
	}
}

// TestTurnstileVerifier_Verify_RespectsTimeout is push.FCMSender's own
// timeout test, same shape: a server that accepts the connection and
// never responds must not hang Verify forever.
func TestTurnstileVerifier_Verify_RespectsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	v := &TurnstileVerifier{secretKey: "k", client: &http.Client{Timeout: 300 * time.Millisecond}, verifyURL: srv.URL}

	start := time.Now()
	err := v.Verify(context.Background(), "some-token", "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Verify to fail against a server that never responds")
	}
	if elapsed > 5*time.Second {
		t.Errorf("expected Verify to fail within a bounded time close to the configured 300ms timeout, took %s", elapsed)
	}
}

// TestTurnstileVerifier_Verify_NeverLogsSecretOrToken is Aturan #26 —
// verified against what a caller could observe (the error message
// itself), not by reading the code. Neither the configured secret nor
// the visitor's response token should ever appear in a returned error
// string, since that string is the one thing most likely to end up in
// a log line at the call site.
func TestTurnstileVerifier_Verify_NeverLogsSecretOrToken(t *testing.T) {
	const secret = "super-secret-turnstile-key-should-never-leak"
	const token = "visitor-response-token-should-never-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false}`))
	}))
	defer srv.Close()

	v := &TurnstileVerifier{secretKey: secret, client: &http.Client{Timeout: 5 * time.Second}, verifyURL: srv.URL}
	err := v.Verify(context.Background(), token, "")
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if strings.Contains(msg, secret) {
		t.Errorf("secret key appeared in error message: %q", msg)
	}
	if strings.Contains(msg, token) {
		t.Errorf("response token appeared in error message: %q", msg)
	}
}
