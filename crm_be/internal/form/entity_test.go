// Internal test package (package form, not form_test) — this file needs
// direct access to generate/parsePublicKey/publicKeyPrefix/
// publicKeyRawBytes, which are deliberately unexported (format details,
// not part of the domain's public surface). Every other _test.go here
// stays package form_test, matching apikey's own convention.
package form

import (
	"encoding/base64"
	"testing"
)

func TestGenerate_ProducesExpectedFormat(t *testing.T) {
	key, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if key[:len(publicKeyPrefix)] != publicKeyPrefix {
		t.Errorf("expected prefix %q, got %q", publicKeyPrefix, key)
	}
	wantLen := len(publicKeyPrefix) + base64.RawURLEncoding.EncodedLen(publicKeyRawBytes)
	if len(key) != wantLen {
		t.Errorf("expected length %d, got %d (%q)", wantLen, len(key), key)
	}
	if !parsePublicKey(key) {
		t.Errorf("parsePublicKey(%q) rejected generate's own output", key)
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		key, err := generate()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[key] {
			t.Fatalf("duplicate public_key generated: %s", key)
		}
		seen[key] = true
	}
}

func TestParsePublicKey_RejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"prefix only":      "pk_",
		"no prefix at all": "abcdefghijklmnopqrstuv",
		"wrong prefix":     "jln_abcdefghijklmnopqrstuv",
		"invalid base64":   "pk_not*valid!base64url",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if parsePublicKey(raw) {
				t.Errorf("parsePublicKey(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

// --- Fields.Validate ---

func TestFieldsValidate_KnownFieldsPass(t *testing.T) {
	if err := DefaultFields().Validate(); err != nil {
		t.Errorf("expected DefaultFields to validate cleanly, got: %v", err)
	}
}

func TestFieldsValidate_UnknownKeyRejected(t *testing.T) {
	f := Fields{"not_a_real_field": {Enabled: true, Label: "x"}}
	if err := f.Validate(); err == nil {
		t.Error("expected an error for an unknown field key")
	}
}

func TestFieldsValidate_RequiredWithoutEnabledRejected(t *testing.T) {
	f := Fields{FieldEmail: {Enabled: false, Required: true, Label: "Email"}}
	if err := f.Validate(); err == nil {
		t.Error("expected an error for required-but-not-enabled")
	}
}

func TestFieldsValidate_EnabledWithoutLabelRejected(t *testing.T) {
	f := Fields{FieldEmail: {Enabled: true, Label: ""}}
	if err := f.Validate(); err == nil {
		t.Error("expected an error for enabled-but-no-label")
	}
}

func TestFieldsValidate_DisabledWithoutLabelIsFine(t *testing.T) {
	f := Fields{FieldCompany: {Enabled: false, Required: false, Label: ""}}
	if err := f.Validate(); err != nil {
		t.Errorf("expected a disabled field with no label to validate cleanly, got: %v", err)
	}
}

// --- DefaultFields ---

func TestDefaultFields_HasAllSixKeys(t *testing.T) {
	f := DefaultFields()
	if len(f) != len(AllFieldKeys) {
		t.Fatalf("expected %d keys, got %d", len(AllFieldKeys), len(f))
	}
	for _, key := range AllFieldKeys {
		if _, ok := f[key]; !ok {
			t.Errorf("expected DefaultFields to include key %q", key)
		}
	}
}

func TestDefaultFields_NameAndPhoneAreRequired(t *testing.T) {
	f := DefaultFields()
	if !f[FieldName].Required || !f[FieldName].Enabled {
		t.Error("expected name to be enabled and required by default")
	}
	if !f[FieldPhone].Required || !f[FieldPhone].Enabled {
		t.Error("expected phone to be enabled and required by default")
	}
}
