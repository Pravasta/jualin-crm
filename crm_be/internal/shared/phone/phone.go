// Package phone normalizes phone numbers to E.164 — Indonesia-first, on
// purpose, not a general E.164 implementation. A full port of
// libphonenumber is a large dependency with metadata for every country
// on earth, for a product whose customers are Indonesian SMBs (TD phase
// 2 §6). The rules below are the entire assumption set; when a customer
// with a non-Indonesian number eventually shows up, ToE164's body is
// what changes — its signature already accepts and returns exactly what
// a real implementation would.
//
// Rules, in order:
//   - Strip everything except digits and a leading '+'.
//   - "+62..."  → already international, validated and returned as-is.
//   - "62..."   → same as above, missing only the '+'.
//   - "0..."    → the domestic trunk prefix; replaced with "+62".
//   - "8..."    → a bare mobile number with neither prefix; "+62" prepended.
//   - Anything else, or a digit count outside [9,13] after the "+62",
//     doesn't parse: ("", false).
//
// A number that fails to parse is NEVER a caller error — rejecting a
// lead over phone format means discarding a customer's data, which is
// unrecoverable (freeze bagian 2.3). Callers store the raw input
// unconditionally and only use PhoneE164 when ok is true.
package phone

import "strings"

const (
	minE164Digits = 9 // after the +62, excluding it
	maxE164Digits = 13
)

// ToE164 normalizes raw to E.164 form. ok is false when raw doesn't
// parse under the rules above — callers must not treat that as an
// error, only as "no normalized form available".
func ToE164(raw string) (e164 string, ok bool) {
	hasPlus := strings.HasPrefix(strings.TrimSpace(raw), "+")
	digits := onlyDigits(raw)
	if digits == "" {
		return "", false
	}

	var national string
	switch {
	case hasPlus && strings.HasPrefix(digits, "62"):
		national = digits[2:]
	case strings.HasPrefix(digits, "62"):
		national = digits[2:]
	case strings.HasPrefix(digits, "0"):
		national = digits[1:]
	case strings.HasPrefix(digits, "8"):
		national = digits
	default:
		return "", false
	}

	if len(national) < minE164Digits || len(national) > maxE164Digits {
		return "", false
	}
	return "+62" + national, true
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
