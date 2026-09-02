package safedial

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// ErrURLNotAllowed is returned by ValidateURL for every rejection reason.
// The caller maps it to a single generic error code
// (webhook_url_not_allowed) — distinguishing "private address" from
// "unresolvable" in the HTTP response hands a customer a tool to map our
// internal network via error messages (TD §7). The wrapped detail is for
// logs only.
var ErrURLNotAllowed = errors.New("webhook url not allowed")

// Validator checks whether a URL is safe to register as an outbound
// webhook target. allowPrivate is the WEBHOOK_ALLOW_PRIVATE_TARGETS
// escape hatch — true only in local development, rejected at boot when
// APP_ENV=production (config.go, Rule #36).
type Validator struct {
	allowPrivate bool
	resolver     *net.Resolver
}

func NewValidator(allowPrivate bool) *Validator {
	return &Validator{allowPrivate: allowPrivate, resolver: net.DefaultResolver}
}

// ValidateURL is the save-time half of TD §3.2's "divalidasi dua kali".
// It parses rawURL, rejects non-http(s) schemes, resolves the host, and
// rejects if ANY resolved address is in a denied range. The send-time
// half — re-resolving and dialing the validated IP directly — is the
// worker's (#102); this package will grow a DialContext for it.
//
// When allowPrivate is set, the IP checks are skipped entirely (localhost
// resolves to 127.0.0.1, which is denied) — scheme and host presence are
// still enforced so a genuinely malformed URL never reaches storage.
func (v *Validator) ValidateURL(ctx context.Context, rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("%w: unparseable", ErrURLNotAllowed)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q is not http(s)", ErrURLNotAllowed, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: no host", ErrURLNotAllowed)
	}

	if v.allowPrivate {
		return nil
	}

	// An IP literal in the URL is checked directly — no DNS round trip,
	// and no chance of the literal and a later lookup disagreeing.
	if addr, err := netip.ParseAddr(host); err == nil {
		if isDeniedIP(addr) {
			return fmt.Errorf("%w: %s is in a denied range", ErrURLNotAllowed, addr)
		}
		return nil
	}

	addrs, err := v.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve %q: %v", ErrURLNotAllowed, host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%w: %q resolves to no addresses", ErrURLNotAllowed, host)
	}
	for _, addr := range addrs {
		if isDeniedIP(addr) {
			return fmt.Errorf("%w: %q resolves to %s, in a denied range", ErrURLNotAllowed, host, addr)
		}
	}
	return nil
}
