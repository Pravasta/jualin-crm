package mailer_test

// End-to-end tests for SMTPMailer against a REAL SMTP server — Mailpit
// via testcontainers, not a mock (Phase 4.6 decision E5). The same
// reasoning as internal/shared/db/dbtest since issue #3: a mock SMTP
// server would never catch a malformed RFC 5322 message, a header that
// doesn't survive the wire, or an encoding that arrives garbled — only
// something that actually parses mail can prove that. Every test here
// reads the message BACK out of Mailpit's own API rather than asserting
// Send returned nil.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/mailer"
)

// mailpitEndpoint is the SMTP address and HTTP API base URL of a Mailpit
// container started once per test binary — container startup cost stays
// off the critical path of each individual test, same pattern as
// dbtest.ConnString.
type mailpitEndpoint struct {
	smtpHost string
	smtpPort int
	apiBase  string
}

func startMailpit(t *testing.T) mailpitEndpoint {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.Run(ctx, "axllent/mailpit:latest",
		testcontainers.WithExposedPorts("1025/tcp", "8025/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("8025/tcp").WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start mailpit testcontainer: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("failed to terminate mailpit container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get mailpit host: %v", err)
	}
	smtpPort, err := container.MappedPort(ctx, "1025/tcp")
	if err != nil {
		t.Fatalf("failed to get mailpit smtp port: %v", err)
	}
	httpPort, err := container.MappedPort(ctx, "8025/tcp")
	if err != nil {
		t.Fatalf("failed to get mailpit http port: %v", err)
	}

	return mailpitEndpoint{
		smtpHost: host,
		smtpPort: int(smtpPort.Num()),
		apiBase:  fmt.Sprintf("http://%s:%d", host, httpPort.Num()),
	}
}

func (e mailpitEndpoint) mailer() *mailer.SMTPMailer {
	return mailer.NewSMTPMailer(mailer.SMTPConfig{
		Host:    e.smtpHost,
		Port:    e.smtpPort,
		From:    "no-reply@jualin.local",
		TLS:     "none", // Mailpit takes no TLS, matching docker-compose.yml's local setup
		Timeout: 10 * time.Second,
	})
}

// mailpitMessageSummary mirrors the fields this test file reads from
// GET /api/v1/messages — Mailpit's list endpoint, deliberately not the
// full shape of its response.
type mailpitMessageSummary struct {
	ID      string
	From    mailpitAddress
	To      []mailpitAddress
	Subject string
}

type mailpitAddress struct {
	Address string
}

type mailpitMessageDetail struct {
	Text string
}

// latestMessage polls Mailpit's list endpoint until at least one message
// has arrived, then returns the newest one — SMTP delivery to Mailpit is
// effectively immediate, but not synchronous with the SMTP transaction
// completing on the wire.
func latestMessage(t *testing.T, apiBase string) mailpitMessageSummary {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(apiBase + "/api/v1/messages")
		if err != nil {
			t.Fatalf("failed to query mailpit messages: %v", err)
		}
		var body struct {
			Messages []mailpitMessageSummary `json:"messages"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to decode mailpit response: %v", err)
		}
		if len(body.Messages) > 0 {
			return body.Messages[0] // Mailpit lists newest first
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("no message arrived at mailpit within 5s")
	return mailpitMessageSummary{}
}

func messageDetail(t *testing.T, apiBase, id string) mailpitMessageDetail {
	t.Helper()
	resp, err := http.Get(apiBase + "/api/v1/message/" + id)
	if err != nil {
		t.Fatalf("failed to fetch mailpit message detail: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var detail mailpitMessageDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode mailpit message detail: %v", err)
	}
	return detail
}

// TestSMTPMailer_MessageActuallyArrives is Phase 4.6 acceptance criterion
// #1: proof the message is sent, delivered, and readable — not just that
// Send returned nil.
func TestSMTPMailer_MessageActuallyArrives(t *testing.T) {
	ep := startMailpit(t)
	m := ep.mailer()

	err := m.Send(context.Background(), mailer.Message{
		To:      "budi@example.com",
		Subject: "Verifikasi email Jualin CRM Anda",
		Body:    "Klik tautan berikut untuk memverifikasi email Anda: http://localhost:3000/verify-email?token=abc123",
	})
	if err != nil {
		t.Fatalf("Send returned an error: %v", err)
	}

	got := latestMessage(t, ep.apiBase)
	if got.Subject != "Verifikasi email Jualin CRM Anda" {
		t.Errorf("expected Subject %q, got %q", "Verifikasi email Jualin CRM Anda", got.Subject)
	}
	if len(got.To) != 1 || got.To[0].Address != "budi@example.com" {
		t.Errorf("expected To=[budi@example.com], got %+v", got.To)
	}

	detail := messageDetail(t, ep.apiBase, got.ID)
	wantBody := "Klik tautan berikut untuk memverifikasi email Anda: http://localhost:3000/verify-email?token=abc123"
	if detail.Text != wantBody+"\r\n" && detail.Text != wantBody {
		t.Errorf("expected body %q, got %q", wantBody, detail.Text)
	}
}

// TestSMTPMailer_UsesMailFromAsSender is acceptance criterion #4:
// MAIL_FROM has existed since Phase 1 and was read by nobody — this
// proves SMTPMailer finally uses it as the actual sender.
func TestSMTPMailer_UsesMailFromAsSender(t *testing.T) {
	ep := startMailpit(t)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{
		Host: ep.smtpHost, Port: ep.smtpPort,
		From: "sender-under-test@jualin.local", TLS: "none", Timeout: 10 * time.Second,
	})

	if err := m.Send(context.Background(), mailer.Message{To: "budi@example.com", Subject: "s", Body: "b"}); err != nil {
		t.Fatalf("Send returned an error: %v", err)
	}

	got := latestMessage(t, ep.apiBase)
	if got.From.Address != "sender-under-test@jualin.local" {
		t.Errorf("expected From=sender-under-test@jualin.local, got %q — MAIL_FROM is not being used as the sender", got.From.Address)
	}
}

// TestSMTPMailer_NonASCIISubjectArrivesIntact is acceptance criterion
// #10, from the receiving end this time (message_internal_test.go
// already proves the encoder's output shape) — Mailpit must decode the
// RFC 2047 header back to the exact original text.
func TestSMTPMailer_NonASCIISubjectArrivesIntact(t *testing.T) {
	ep := startMailpit(t)
	m := ep.mailer()

	want := "Sudah — kami kirim ulang"
	if err := m.Send(context.Background(), mailer.Message{To: "budi@example.com", Subject: want, Body: "b"}); err != nil {
		t.Fatalf("Send returned an error: %v", err)
	}

	got := latestMessage(t, ep.apiBase)
	if got.Subject != want {
		t.Errorf("expected Subject %q to arrive intact, got %q", want, got.Subject)
	}
}

// TestSMTPMailer_UnreachableServerFailsWithinTimeout is acceptance
// criterion #7: Send runs synchronously in the request path, so a server
// that stops answering mid-conversation must not hang the request. A
// local listener that accepts the connection and then never writes a
// byte reproduces that without needing an actual network blackhole.
func TestSMTPMailer_UnreachableServerFailsWithinTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start local listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Accept the connection and never write the SMTP greeting —
			// simulates a server that has stopped responding mid-handshake.
			_ = conn // held open deliberately, never closed by this goroutine
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{
		Host: "127.0.0.1", Port: addr.Port,
		From: "no-reply@jualin.local", TLS: "none", Timeout: 500 * time.Millisecond,
	})

	start := time.Now()
	err = m.Send(context.Background(), mailer.Message{To: "budi@example.com", Subject: "s", Body: "b"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Send to fail against a server that never responds, got nil error")
	}
	// Generous upper bound (well above the 500ms configured timeout) —
	// the point being proven is "bounded", not a tight latency budget.
	if elapsed > 5*time.Second {
		t.Errorf("expected Send to fail within a bounded time close to the configured 500ms timeout, took %s", elapsed)
	}
}
