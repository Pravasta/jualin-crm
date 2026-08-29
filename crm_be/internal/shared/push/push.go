// Package push defines the interface every push-sending flow in this
// codebase uses, and a NoopSender implementation for development and
// test — same shape as internal/shared/mailer (Phase 4.6), which itself
// followed the reasoning an interface with one real implementation
// normally violates Rule #27, justified here because the second
// implementation (FCMSender) is built in the same issue, not deferred.
//
// Sends always happen after the triggering transaction commits (Rule
// #32) — see internal/lead.Usecase.pushAssignmentNotification. A send
// failure is logged structurally; it never rolls back work that already
// committed, and freeze bagian A3 has already established the
// notification row (not the push) as source of truth.
package push

import (
	"context"
	"log/slog"
)

// Message is provider-agnostic. Data becomes the FCM "data payload" —
// used for deeplinking on the client (Phase 5 TD §10), never rendered
// as visible text itself.
type Message struct {
	Token string
	Title string
	Body  string
	Data  map[string]string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// NoopSender records that a send was attempted via the request-scoped
// logger instead of sending it. Used in development (PUSH_PROVIDER=none)
// and in every test that doesn't specifically exercise FCMSender — no
// test in this codebase depends on network access or a real device.
//
// Deliberately never logs msg.Token or msg.Data — a device token
// identifies one physical installation and is treated as a credential
// (Phase 5 TD §9.5, Rule #26), the same reasoning LogMailer already
// applies to never logging a raw secret.
type NoopSender struct {
	Logger *slog.Logger
}

func NewNoopSender(logger *slog.Logger) *NoopSender {
	return &NoopSender{Logger: logger}
}

func (s *NoopSender) Send(_ context.Context, msg Message) error {
	s.Logger.Info("push (not sent — NoopSender)", "title", msg.Title)
	return nil
}
