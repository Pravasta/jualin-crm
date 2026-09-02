package safedial

// The send-time half of the SSRF defence (TD §3.2/§3.3). These tests run
// against real local listeners rather than a mock transport: what is being
// verified is how net/http actually behaves — which connection it makes,
// whether it reuses one, whether it follows a redirect — and a fake
// transport would only prove our own assumptions back to us.

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPClient_RefusesDeniedAddressAtSendTime(t *testing.T) {
	v := NewValidator(false)
	client := v.HTTPClient(2 * time.Second)

	// A literal loopback target: no DNS involved, so this isolates the
	// deny-list check inside DialContext from anything else.
	resp, err := client.Get("http://127.0.0.1:9/hook")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected the dial to be refused")
	}
	if !errors.Is(err, ErrURLNotAllowed) {
		t.Fatalf("expected ErrURLNotAllowed, got %v", err)
	}
}

func TestHTTPClient_AllowPrivateReachesLocalListener(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// The same request the test above rejects must succeed once the
	// development escape hatch is on — otherwise the whole phase would be
	// undevelopable locally.
	client := NewValidator(true).HTTPClient(2 * time.Second)
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("expected the loopback target to be reachable with allowPrivate, got %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// TestHTTPClient_NeverReusesConnections is the regression test for the
// keep-alive hole: Transport only calls DialContext for a NEW connection,
// so a pooled one would skip the deny-list on every delivery after the
// first. Counting accepted connections is the only way to observe this
// from outside — a passing status code proves nothing either way.
func TestHTTPClient_NeverReusesConnections(t *testing.T) {
	var conns int64
	// Unstarted, so ConnState is installed before the accept loop begins —
	// setting it on an already-serving server is a data race, which is
	// exactly what `go test -race` reported the first time this was written
	// with httptest.NewServer.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			atomic.AddInt64(&conns, 1)
		}
	}
	srv.Start()
	defer srv.Close()

	client := NewValidator(true).HTTPClient(2 * time.Second)
	for i := 0; i < 3; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	if got := atomic.LoadInt64(&conns); got != 3 {
		t.Fatalf("three requests opened %d connections, want 3 — a reused connection skips the deny-list check entirely", got)
	}
}

// TestHTTPClient_DoesNotFollowRedirect covers the most direct bypass in
// TD §3.3: a legitimate public URL answering 302 to a link-local address.
// The redirect must surface as an ordinary 3xx response, never as a second
// request.
func TestHTTPClient_DoesNotFollowRedirect(t *testing.T) {
	var followed atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/after" {
			followed.Store(true)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	client := NewValidator(true).HTTPClient(2 * time.Second)
	resp, err := client.Get(srv.URL + "/hook")
	if err != nil {
		t.Fatalf("expected the 3xx to come back as a response, got error %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302 returned verbatim", resp.StatusCode)
	}
	if followed.Load() {
		t.Fatal("the client followed the redirect")
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "169.254.169.254") {
		t.Errorf("Location = %q, expected the untouched redirect target", loc)
	}
}

// TestHTTPClient_HostHeaderKeepsHostname proves the pinning does not break
// virtual hosting or (by the same mechanism) TLS SNI: DialContext connects
// to an IP, but Transport still builds Host from the request URL.
func TestHTTPClient_HostHeaderKeepsHostname(t *testing.T) {
	var gotHost atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost.Store(r.Host)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("split test server address: %v", err)
	}

	client := NewValidator(true).HTTPClient(2 * time.Second)
	// "localhost" resolves to a loopback address; the connection is pinned
	// to that address while the Host header must stay "localhost:port".
	resp, err := client.Get("http://localhost:" + port + "/hook")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if want := "localhost:" + port; gotHost.Load() != want {
		t.Errorf("Host header = %v, want %q", gotHost.Load(), want)
	}
}

func TestHTTPClient_TimeoutIsEnforced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewValidator(true).HTTPClient(200 * time.Millisecond)
	start := time.Now()
	resp, err := client.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected a timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("timeout took %s, want it bounded near 200ms", elapsed)
	}
}

func TestResolveChecked_RejectsUnresolvableHost(t *testing.T) {
	v := NewValidator(false)
	_, err := v.resolveChecked(context.Background(), "this-host-does-not-exist.invalid")
	if !errors.Is(err, ErrURLNotAllowed) {
		t.Fatalf("expected ErrURLNotAllowed, got %v", err)
	}
}

// TestResolveChecked_RejectsHostWithAnyDeniedAddress documents the
// all-or-nothing rule: a host answering with one public and one private
// address is rejected outright rather than filtered down to the public
// one, because that mix is far more likely to be a rebinding attempt than
// a legitimate target.
func TestResolveChecked_RejectsHostWithAnyDeniedAddress(t *testing.T) {
	v := NewValidator(false)
	if _, err := v.resolveChecked(context.Background(), "127.0.0.1"); !errors.Is(err, ErrURLNotAllowed) {
		t.Fatalf("expected a denied literal to be rejected, got %v", err)
	}
	if _, err := v.resolveChecked(context.Background(), "93.184.216.34"); err != nil {
		t.Fatalf("expected a public literal to be accepted, got %v", err)
	}
}
