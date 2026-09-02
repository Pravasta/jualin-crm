// Package safedial guards outbound HTTP calls to targets a customer
// controls (Phase 7 — outbound webhook, TD §3). A webhook endpoint URL is
// typed by a customer; the server then calls it. Without this package
// that endpoint is a customer-controlled request proxy into our own
// network — SSRF, and not a theoretical one: 169.254.169.254 returns
// cloud instance credentials on nearly every provider.
//
// This file is the deny-list — the single security-critical decision of
// the phase. It is tested as a table over every range in TD §3.1
// (denylist_test.go), IPv4 and IPv6, not a handful of examples.
package safedial

import "net/netip"

// deniedPrefixes are ranges no legitimate webhook target ever lives in,
// beyond what netip.Addr's own predicate methods already cover (loopback,
// private, link-local, multicast, unspecified — see isDeniedIP). Every
// entry here is a range those methods miss.
var deniedPrefixes = []netip.Prefix{
	// IPv4
	netip.MustParsePrefix("0.0.0.0/8"),       // "this host on this network" (RFC 1122) — broader than IsUnspecified's single 0.0.0.0
	netip.MustParsePrefix("100.64.0.0/10"),   // CGNAT shared address space (RFC 6598) — no stdlib predicate
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments (RFC 6890)
	netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1, documentation (RFC 5737)
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking (RFC 2544)
	netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved class E, and 255.255.255.255 broadcast (RFC 1112)

	// IPv6
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64 (RFC 6052) — embeds an IPv4 address, which could be private
	netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64 (RFC 8215)
	netip.MustParsePrefix("100::/64"),       // discard-only (RFC 6666)
	netip.MustParsePrefix("2001::/23"),      // IETF protocol assignments — includes Teredo 2001::/32
	netip.MustParsePrefix("2001:db8::/32"),  // documentation (RFC 3849)
}

// isDeniedIP reports whether addr must never be the target of an outbound
// webhook. An invalid address is denied — a caller that can't produce a
// valid IP has no business dialing.
//
// addr.Unmap() FIRST is not optional: without it, ::ffff:169.254.169.254
// (an IPv4-mapped IPv6 address) sails past every IsPrivate/IsLinkLocal
// check because those are evaluated against the IPv6 form. This is the
// single most likely bypass, and it's covered explicitly in the test.
func isDeniedIP(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	addr = addr.Unmap()

	if addr.IsLoopback() ||
		addr.IsPrivate() || // 10/8, 172.16/12, 192.168/16, fc00::/7
		addr.IsUnspecified() || // 0.0.0.0, ::
		addr.IsLinkLocalUnicast() || // 169.254/16, fe80::/10
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() {
		return true
	}

	for _, p := range deniedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
