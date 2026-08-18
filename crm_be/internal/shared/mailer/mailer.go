// Package mailer defines the interface every email-sending flow in this
// codebase uses, and a LogMailer implementation for development and
// test.
//
// An interface with a single implementation usually violates Rule #27
// (no abstraction before a second real user exists). It's justified here
// because the second implementation — a real provider — is already known
// to be coming (see docs/STATUS.md's lead-time items: domain + SPF/DKIM/
// DMARC), and without this interface Phase 1 would be blocked on a
// decision (which provider) that's outside this phase's control.
//
// Sends always happen after the triggering transaction commits (Rule
// #32) — see internal/auth.Service.Register for the pattern. A send
// failure is logged structurally; it never rolls back work that already
// committed.
package mailer

import (
	"context"
	"log/slog"
)

// Message is provider-agnostic — no HTML templating, attachments, or
// provider-specific fields. Phase 1 only sends transactional plaintext
// email.
type Message struct {
	To      string
	Subject string
	Body    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// LogMailer records the message via the request-scoped logger instead of
// sending it. Used in development and in every test — no test in this
// codebase depends on network access or a real inbox.
type LogMailer struct {
	Logger *slog.Logger
}

func NewLogMailer(logger *slog.Logger) *LogMailer {
	return &LogMailer{Logger: logger}
}

func (m *LogMailer) Send(_ context.Context, msg Message) error {
	m.Logger.Info("email (not sent — LogMailer)",
		"to", msg.To,
		"subject", msg.Subject,
		"body", msg.Body,
	)
	return nil
}
