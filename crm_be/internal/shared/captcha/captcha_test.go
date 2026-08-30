package captcha_test

import (
	"context"
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/captcha"
)

func TestNoopVerifier_AlwaysSucceeds(t *testing.T) {
	v := captcha.NoopVerifier{}
	cases := []struct{ token, remoteIP string }{
		{"", ""},
		{"any-token", "203.0.113.5"},
		{"not even a real turnstile token", ""},
	}
	for _, c := range cases {
		if err := v.Verify(context.Background(), c.token, c.remoteIP); err != nil {
			t.Errorf("expected NoopVerifier to always succeed, got: %v", err)
		}
	}
}
