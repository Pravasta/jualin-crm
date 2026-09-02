package safedial

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// dialTimeout bounds the TCP connect to one address. The overall request
// is already bounded by the caller's http.Client.Timeout; this only stops
// a single unreachable address in a multi-address host from consuming the
// whole budget before the others are tried.
const dialTimeout = 5 * time.Second

// HTTPClient returns the client the worker delivers with — the send-time
// half of TD §3.2's "divalidasi dua kali". Three properties make it safe,
// and all three are load-bearing:
//
//  1. Resolution happens ONCE, inside DialContext, and the connection is
//     made to an address from that same lookup. Validating a hostname and
//     then handing the hostname to http.Client would mean two independent
//     DNS lookups — one to check, one to connect — which an attacker can
//     make answer differently (TOCTOU).
//
//  2. Keep-alives are DISABLED. This is not a performance choice. Transport
//     only calls DialContext when it needs a NEW connection, so a pooled
//     connection would skip the deny-list entirely on every delivery after
//     the first — which is exactly the common case for an endpoint that
//     receives events regularly, and would leave the DNS-rebinding window
//     TD §3.2 exists to close wide open. One connection per delivery costs
//     a handshake and buys the guarantee that every send is checked.
//
//  3. Redirects are never followed (§3.3). CheckRedirect returns
//     http.ErrUseLastResponse rather than an error, so the 3xx arrives at
//     the caller as an ordinary response and flows through the same status
//     mapping as everything else — one code path, not a special case. The
//     worker treats 3xx as a permanent failure (TD §4.3).
//
// The Host header and TLS SNI still carry the original hostname: Transport
// builds those from the request URL, independently of what DialContext
// connected to. Certificate verification therefore remains correct.
func (v *Validator) HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			DialContext:       v.dialValidated,
			DisableKeepAlives: true,
			// No proxy: a proxy would defeat the pinning above by
			// resolving the target itself, somewhere we cannot check.
			Proxy:                 nil,
			TLSHandshakeTimeout:   dialTimeout,
			ResponseHeaderTimeout: timeout,
		},
	}
}

// dialValidated resolves addr's host, rejects the whole attempt if ANY
// resolved address is denied, then connects to those addresses.
//
// Rejecting on any denied address rather than filtering them out is
// deliberate: a host that resolves to both a public and a private address
// is far more likely to be a rebinding attack than a legitimate target,
// and silently using the public one would hide that.
func (v *Validator) dialValidated(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("%w: unparseable address %q", ErrURLNotAllowed, addr)
	}

	addrs, err := v.resolveChecked(ctx, host)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: dialTimeout}

	// Every validated address is tried in turn. Dialing only the first
	// would fail the whole delivery whenever a host's first A record is
	// down, even though the others answer — which is what an ordinary
	// dialer would have handled for us before we took resolution over.
	var lastErr error
	for _, a := range addrs {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(a.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("dial %q: %w", host, lastErr)
}

// resolveChecked returns the addresses to dial for host, after the
// deny-list. When allowPrivate is set the check is skipped but resolution
// still happens here, so the pinning in dialValidated behaves identically
// in development and production — the escape hatch changes what is
// allowed, never the shape of the code path.
func (v *Validator) resolveChecked(ctx context.Context, host string) ([]netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		if !v.allowPrivate && isDeniedIP(addr) {
			return nil, fmt.Errorf("%w: %s is in a denied range", ErrURLNotAllowed, addr)
		}
		return []netip.Addr{addr}, nil
	}

	addrs, err := v.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot resolve %q: %v", ErrURLNotAllowed, host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%w: %q resolves to no addresses", ErrURLNotAllowed, host)
	}
	if !v.allowPrivate {
		for _, addr := range addrs {
			if isDeniedIP(addr) {
				return nil, fmt.Errorf("%w: %q resolves to %s, in a denied range", ErrURLNotAllowed, host, addr)
			}
		}
	}
	return addrs, nil
}
