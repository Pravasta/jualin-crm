package safedial

import (
	"context"
	"errors"
	"testing"
)

func TestValidator_ValidateURL_RejectsBadScheme(t *testing.T) {
	v := NewValidator(false)
	for _, raw := range []string{
		"ftp://example.com/hook",
		"file:///etc/passwd",
		"gopher://example.com",
		"ws://example.com",
		"example.com/hook", // no scheme -> url.Parse gives empty Scheme
	} {
		if err := v.ValidateURL(context.Background(), raw); !errors.Is(err, ErrURLNotAllowed) {
			t.Errorf("%q: expected ErrURLNotAllowed, got %v", raw, err)
		}
	}
}

func TestValidator_ValidateURL_RejectsMissingHost(t *testing.T) {
	v := NewValidator(false)
	if err := v.ValidateURL(context.Background(), "https:///just-a-path"); !errors.Is(err, ErrURLNotAllowed) {
		t.Errorf("expected ErrURLNotAllowed for hostless URL, got %v", err)
	}
}

func TestValidator_ValidateURL_RejectsPrivateIPLiteral(t *testing.T) {
	v := NewValidator(false)
	for _, raw := range []string{
		"http://127.0.0.1/hook",
		"https://10.0.0.5:8443/hook",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/hook",
		"http://[::ffff:127.0.0.1]/hook", // IPv4-mapped — the bypass
	} {
		if err := v.ValidateURL(context.Background(), raw); !errors.Is(err, ErrURLNotAllowed) {
			t.Errorf("%q: expected ErrURLNotAllowed, got %v", raw, err)
		}
	}
}

func TestValidator_ValidateURL_AllowsPublicIPLiteral(t *testing.T) {
	v := NewValidator(false)
	if err := v.ValidateURL(context.Background(), "https://8.8.8.8/hook"); err != nil {
		t.Errorf("expected public IP literal to pass, got %v", err)
	}
}

func TestValidator_ValidateURL_RejectsUnresolvableHost(t *testing.T) {
	v := NewValidator(false)
	// .invalid is reserved by RFC 2606 to never resolve.
	err := v.ValidateURL(context.Background(), "https://this-host-does-not-exist.invalid/hook")
	if !errors.Is(err, ErrURLNotAllowed) {
		t.Errorf("expected ErrURLNotAllowed for unresolvable host, got %v", err)
	}
}

func TestValidator_ValidateURL_AllowPrivateSkipsIPChecks(t *testing.T) {
	v := NewValidator(true)
	// A private literal that would be rejected with allowPrivate=false.
	if err := v.ValidateURL(context.Background(), "http://127.0.0.1:9099/hook"); err != nil {
		t.Errorf("expected allowPrivate to permit loopback, got %v", err)
	}
	if err := v.ValidateURL(context.Background(), "http://localhost:9099/hook"); err != nil {
		t.Errorf("expected allowPrivate to permit localhost, got %v", err)
	}
	// Scheme is still enforced even with allowPrivate.
	if err := v.ValidateURL(context.Background(), "ftp://127.0.0.1/hook"); !errors.Is(err, ErrURLNotAllowed) {
		t.Errorf("expected scheme still enforced under allowPrivate, got %v", err)
	}
}

func TestValidator_ValidateURL_AllowsPublicHost(t *testing.T) {
	v := NewValidator(false)
	// example.com is stable and public; this test makes a real DNS query.
	if err := v.ValidateURL(context.Background(), "https://example.com/hook"); err != nil {
		t.Errorf("expected example.com to pass, got %v", err)
	}
}
