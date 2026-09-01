package safedial

import (
	"net/netip"
	"testing"
)

// TestIsDeniedIP_DeniedRanges is the phase's single most important test —
// a table over every range TD §3.1 names, checked at a representative
// address inside each. A gap here is an SSRF hole, so it's exhaustive by
// design, not a handful of examples.
func TestIsDeniedIP_DeniedRanges(t *testing.T) {
	denied := []struct {
		name string
		ip   string
	}{
		// IPv4 — loopback
		{"IPv4 loopback 127.0.0.1", "127.0.0.1"},
		{"IPv4 loopback 127.255.255.254", "127.255.255.254"},
		// IPv4 — private (RFC 1918)
		{"IPv4 private 10.0.0.1", "10.0.0.1"},
		{"IPv4 private 172.16.0.1", "172.16.0.1"},
		{"IPv4 private 172.31.255.255", "172.31.255.255"},
		{"IPv4 private 192.168.1.1", "192.168.1.1"},
		// IPv4 — link-local, incl. cloud metadata
		{"IPv4 link-local 169.254.0.1", "169.254.0.1"},
		{"IPv4 cloud metadata 169.254.169.254", "169.254.169.254"},
		// IPv4 — CGNAT (RFC 6598)
		{"IPv4 CGNAT 100.64.0.1", "100.64.0.1"},
		{"IPv4 CGNAT 100.127.255.255", "100.127.255.255"},
		// IPv4 — this-network / unspecified
		{"IPv4 unspecified 0.0.0.0", "0.0.0.0"},
		{"IPv4 this-network 0.1.2.3", "0.1.2.3"},
		// IPv4 — reserved / broadcast
		{"IPv4 broadcast 255.255.255.255", "255.255.255.255"},
		{"IPv4 reserved class E 240.0.0.1", "240.0.0.1"},
		// IPv4 — documentation / benchmarking / protocol
		{"IPv4 TEST-NET-1 192.0.2.1", "192.0.2.1"},
		{"IPv4 TEST-NET-2 198.51.100.1", "198.51.100.1"},
		{"IPv4 TEST-NET-3 203.0.113.1", "203.0.113.1"},
		{"IPv4 benchmarking 198.18.0.1", "198.18.0.1"},
		{"IPv4 protocol assignments 192.0.0.1", "192.0.0.1"},
		// IPv4 — multicast
		{"IPv4 multicast 224.0.0.1", "224.0.0.1"},
		{"IPv4 multicast 239.255.255.255", "239.255.255.255"},

		// IPv6 — loopback / unspecified
		{"IPv6 loopback ::1", "::1"},
		{"IPv6 unspecified ::", "::"},
		// IPv6 — link-local
		{"IPv6 link-local fe80::1", "fe80::1"},
		{"IPv6 link-local febf::ffff", "febf::ffff"},
		// IPv6 — unique local (fc00::/7)
		{"IPv6 ULA fc00::1", "fc00::1"},
		{"IPv6 ULA fd12:3456::1", "fd12:3456::1"},
		// IPv6 — multicast
		{"IPv6 multicast ff02::1", "ff02::1"},
		{"IPv6 interface-local multicast ff01::1", "ff01::1"},
		// IPv6 — documentation / NAT64 / discard / protocol
		{"IPv6 documentation 2001:db8::1", "2001:db8::1"},
		{"IPv6 NAT64 64:ff9b::7f00:1", "64:ff9b::7f00:1"},
		{"IPv6 local NAT64 64:ff9b:1::1", "64:ff9b:1::1"},
		{"IPv6 discard-only 100::1", "100::1"},
		{"IPv6 Teredo 2001:0::1", "2001:0:0:0:0:0:0:1"},

		// The bypass that matters most: IPv4-mapped IPv6. Without
		// Unmap() first, every check above is evaluated against the
		// IPv6 form and passes.
		{"IPv4-mapped loopback ::ffff:127.0.0.1", "::ffff:127.0.0.1"},
		{"IPv4-mapped metadata ::ffff:169.254.169.254", "::ffff:169.254.169.254"},
		{"IPv4-mapped private ::ffff:10.0.0.1", "::ffff:10.0.0.1"},
	}

	for _, c := range denied {
		t.Run(c.name, func(t *testing.T) {
			addr, err := netip.ParseAddr(c.ip)
			if err != nil {
				t.Fatalf("bad test address %q: %v", c.ip, err)
			}
			if !isDeniedIP(addr) {
				t.Errorf("expected %s to be DENIED, got allowed", c.ip)
			}
		})
	}
}

// TestIsDeniedIP_AllowedRanges proves the deny-list isn't so broad it
// rejects real targets — public unicast addresses must pass.
func TestIsDeniedIP_AllowedRanges(t *testing.T) {
	allowed := []struct {
		name string
		ip   string
	}{
		{"IPv4 public 1.1.1.1", "1.1.1.1"},
		{"IPv4 public 8.8.8.8", "8.8.8.8"},
		{"IPv4 public 93.184.216.34", "93.184.216.34"}, // example.com
		{"IPv4 just outside CGNAT 100.63.255.255", "100.63.255.255"},
		{"IPv4 just outside CGNAT 100.128.0.0", "100.128.0.0"},
		{"IPv4 just outside private 172.15.255.255", "172.15.255.255"},
		{"IPv4 just outside private 172.32.0.0", "172.32.0.0"},
		{"IPv4 just outside link-local 169.253.255.255", "169.253.255.255"},
		{"IPv4 just outside link-local 169.255.0.0", "169.255.0.0"},
		{"IPv6 public 2606:4700:4700::1111", "2606:4700:4700::1111"}, // cloudflare
		{"IPv6 public 2001:4860:4860::8888", "2001:4860:4860::8888"}, // google
		{"IPv4-mapped public ::ffff:8.8.8.8", "::ffff:8.8.8.8"},
	}

	for _, c := range allowed {
		t.Run(c.name, func(t *testing.T) {
			addr, err := netip.ParseAddr(c.ip)
			if err != nil {
				t.Fatalf("bad test address %q: %v", c.ip, err)
			}
			if isDeniedIP(addr) {
				t.Errorf("expected %s to be ALLOWED, got denied", c.ip)
			}
		})
	}
}

func TestIsDeniedIP_InvalidAddrDenied(t *testing.T) {
	if !isDeniedIP(netip.Addr{}) {
		t.Error("expected the zero Addr to be denied")
	}
}
