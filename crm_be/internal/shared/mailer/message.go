package mailer

import (
	"fmt"
	"mime"
	"strings"
	"time"
)

// buildRFC5322 renders msg as a complete RFC 5322 message ready to hand
// to an SMTP DATA command. Split out as a pure function (Phase 4.6 TD
// §3) so its format — headers, encoding, header-injection defense — can
// be tested without a network, a server, or a container. SMTPMailer.Send
// is the only caller.
func buildRFC5322(from string, msg Message, now time.Time) ([]byte, error) {
	if err := rejectHeaderInjection("from", from); err != nil {
		return nil, err
	}
	if err := rejectHeaderInjection("to", msg.To); err != nil {
		return nil, err
	}
	if err := rejectHeaderInjection("subject", msg.Subject); err != nil {
		return nil, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", now.Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	// msg.Body is never checked for \r\n — it comes after the header/body
	// separator above, so a newline there is message content, not a new
	// header.
	b.WriteString(msg.Body)

	return []byte(b.String()), nil
}

// rejectHeaderInjection refuses (rather than silently stripping) any
// value bound for a header line if it contains \r or \n. msg.To in
// particular is user-supplied (registration and invitation forms) — an
// address like "victim@x.com\r\nBcc: attacker@y.com" would otherwise
// smuggle an extra header into the message. Stripping the characters
// would send to an address the caller never actually asked for, silently;
// refusing surfaces an error the caller already has a path to handle
// (Rule #32's structured-error logging, in place since Phase 1).
func rejectHeaderInjection(field, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("mailer: %s contains a line break, refusing to build message", field)
	}
	return nil
}
