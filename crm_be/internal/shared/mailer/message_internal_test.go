package mailer

// package mailer (not mailer_test) — buildRFC5322 is unexported by
// design (Phase 4.6 TD §3: split out purely so message format can be
// tested without a network, not meant as public API). Every other test
// in this package stays external (smtp_test.go); this mirrors the same
// isolated exception ratelimit's limiter_internal_test.go and
// apikey's entity_test.go already set precedent for.

import (
	"strings"
	"testing"
	"time"
)

func TestBuildRFC5322_HeadersAndBody(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	raw, err := buildRFC5322("no-reply@jualin.local", Message{
		To:      "budi@example.com",
		Subject: "Verifikasi email Jualin CRM Anda",
		Body:    "Klik tautan berikut: http://localhost:3000/verify-email?token=abc",
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := string(raw)
	header, body, ok := strings.Cut(msg, "\r\n\r\n")
	if !ok {
		t.Fatalf("expected header/body separated by a blank line, got: %q", msg)
	}

	wantHeaders := []string{
		"From: no-reply@jualin.local",
		"To: budi@example.com",
		"Subject: Verifikasi email Jualin CRM Anda",
		"Date: Sun, 30 Aug 2026 12:00:00 +0000",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	for _, want := range wantHeaders {
		if !strings.Contains(header, want) {
			t.Errorf("expected header block to contain %q, got:\n%s", want, header)
		}
	}

	if body != "Klik tautan berikut: http://localhost:3000/verify-email?token=abc" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestBuildRFC5322_RejectsHeaderInjectionInTo(t *testing.T) {
	cases := []string{
		"victim@x.com\r\nBcc: attacker@y.com",
		"victim@x.com\nBcc: attacker@y.com",
		"victim@x.com\rBcc: attacker@y.com",
	}
	for _, to := range cases {
		_, err := buildRFC5322("no-reply@jualin.local", Message{To: to, Subject: "s", Body: "b"}, time.Now())
		if err == nil {
			t.Errorf("expected header injection in To %q to be rejected, got nil error", to)
		}
	}
}

func TestBuildRFC5322_RejectsHeaderInjectionInSubjectAndFrom(t *testing.T) {
	_, err := buildRFC5322("no-reply@jualin.local", Message{
		To: "budi@example.com", Subject: "Halo\r\nBcc: attacker@y.com", Body: "b",
	}, time.Now())
	if err == nil {
		t.Error("expected header injection in Subject to be rejected, got nil error")
	}

	_, err = buildRFC5322("no-reply@jualin.local\r\nBcc: attacker@y.com", Message{
		To: "budi@example.com", Subject: "s", Body: "b",
	}, time.Now())
	if err == nil {
		t.Error("expected header injection in From to be rejected, got nil error")
	}
}

func TestBuildRFC5322_ASCIISubjectNotEncoded(t *testing.T) {
	for _, subject := range []string{
		"Verifikasi email Jualin CRM Anda",
		"Anda diundang bergabung di Jualin CRM",
		"Reset password Jualin CRM Anda",
	} {
		raw, err := buildRFC5322("no-reply@jualin.local", Message{To: "b@example.com", Subject: subject, Body: "x"}, time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "Subject: " + subject + "\r\n"
		if !strings.Contains(string(raw), want) {
			t.Errorf("expected today's ASCII subject %q to appear unencoded, got:\n%s", subject, raw)
		}
	}
}

func TestBuildRFC5322_NonASCIISubjectEncoded(t *testing.T) {
	raw, err := buildRFC5322("no-reply@jualin.local", Message{
		To: "b@example.com", Subject: "Sudah — kami kirim ulang", Body: "x",
	}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(raw), "Subject: =?utf-8?q?") {
		t.Errorf("expected a non-ASCII subject to be RFC 2047 encoded, got:\n%s", raw)
	}
	if strings.Contains(string(raw), "Subject: Sudah — kami") {
		t.Error("expected the raw em-dash to NOT appear unencoded in the Subject header")
	}
}

func TestBuildRFC5322_BodyNewlinesAreNotHeaders(t *testing.T) {
	raw, err := buildRFC5322("no-reply@jualin.local", Message{
		To: "b@example.com", Subject: "s",
		Body: "Baris pertama.\n\nBaris kedua setelah baris kosong.",
	}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v — a newline in Body must never be treated as a header injection attempt", err)
	}
	if !strings.Contains(string(raw), "Baris pertama.\n\nBaris kedua setelah baris kosong.") {
		t.Errorf("expected body newlines to be preserved verbatim, got:\n%s", raw)
	}
}
