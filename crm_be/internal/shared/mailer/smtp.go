package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// SMTPConfig holds everything SMTPMailer needs to reach a server. Every
// field maps directly to one SMTP_* env var (internal/shared/config) —
// this type has no behavior of its own beyond what NewSMTPMailer does
// with it.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	// From becomes every message's From: header — the env var this
	// finally puts to use. MAIL_FROM has existed since Phase 1 and was
	// never read by anything until this type.
	From string
	// TLS is "starttls" or "none" — config.Validate already rejects any
	// other value, and rejects "none" outright when APP_ENV=production
	// (Phase 4.6 decision E3). Implicit TLS (port 465) is not supported;
	// none of the three provider candidates in freeze.md bagian 7
	// require it.
	TLS string
	// Timeout bounds the ENTIRE SMTP conversation from dial to QUIT, not
	// just the dial — Send runs synchronously in the request path
	// (internal/auth.Usecase.sendVerificationEmail and friends), so a
	// server that stops answering mid-conversation must not hang the
	// HTTP request that triggered it.
	Timeout time.Duration
}

// SMTPMailer sends mail over a real SMTP connection built by hand rather
// than smtp.SendMail — see docs/phases/04.6-email-delivery/td.md §4 for
// why: SendMail has no timeout at all, and that's incompatible with
// being called synchronously from an HTTP handler.
type SMTPMailer struct {
	cfg  SMTPConfig
	addr string
	// now is overridable only from this package's own tests, matching
	// the pattern ratelimit.FixedWindow and auth.LoginLimiter already use
	// (Phase 4.5 #58) — production always gets time.Now.
	now func() time.Time
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{
		cfg:  cfg,
		addr: net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port)),
		now:  time.Now,
	}
}

func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	raw, err := buildRFC5322(m.cfg.From, msg, m.now())
	if err != nil {
		return err
	}

	deadline := time.Now().Add(m.cfg.Timeout)

	dialer := &net.Dialer{Timeout: m.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return fmt.Errorf("mailer: dial %s: %w", m.addr, err)
	}
	defer func() { _ = conn.Close() }()

	// One deadline for the whole remaining conversation — not per
	// operation. A server that answers one byte a second still gets cut
	// off on schedule instead of stalling operation by operation.
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("mailer: set deadline: %w", err)
	}

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("mailer: create smtp client: %w", err)
	}
	defer func() { _ = c.Close() }()

	if m.cfg.TLS == "starttls" {
		ok, _ := c.Extension("STARTTLS")
		if !ok {
			return fmt.Errorf("mailer: server %s does not advertise STARTTLS", m.addr)
		}
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("mailer: starttls: %w", err)
		}
	}

	if m.cfg.Username != "" || m.cfg.Password != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := c.Auth(auth); err != nil {
			// The underlying error is never wrapped here — net/smtp can
			// echo server responses that include auth details. Rule #26:
			// the password must never reach a log or an error message.
			return errors.New("mailer: smtp authentication failed")
		}
	}

	if err := c.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("mailer: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("mailer: RCPT TO: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("mailer: write message body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: close message body: %w", err)
	}

	return c.Quit()
}
