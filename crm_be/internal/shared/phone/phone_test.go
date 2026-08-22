package phone_test

import (
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/phone"
)

func TestToE164(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{"domestic trunk prefix", "0812-3456-7890", "+6281234567890", true},
		{"already E.164 with plus and spaces", "+62 812 3456 7890", "+6281234567890", true},
		{"international without plus", "62812 3456 7890", "+6281234567890", true},
		{"bare mobile number", "812 3456 7890", "+6281234567890", true},
		{"too short to be a real number", "1234", "", false},
		{"empty string", "", "", false},
		{"letters only", "not-a-number", "", false},
		{"too many digits", "0812345678901234567", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := phone.ToE164(c.raw)
			if ok != c.ok {
				t.Fatalf("ToE164(%q): expected ok=%v, got ok=%v (value=%q)", c.raw, c.ok, ok, got)
			}
			if ok && got != c.want {
				t.Errorf("ToE164(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

// TestToE164_UnparseableIsNeverAnError documents the contract callers
// depend on: a phone number that doesn't parse is not a rejection
// signal, just an absence of a normalized form (freeze bagian 2.3 —
// never discard a lead over contact format).
func TestToE164_UnparseableIsNeverAnError(t *testing.T) {
	_, ok := phone.ToE164("this is not a phone number")
	if ok {
		t.Fatal("expected ok=false for unparseable input")
	}
}
