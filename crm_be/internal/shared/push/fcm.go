package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// fcmMessagingScope is the one OAuth2 scope FCMSender ever requests —
// narrower than the Firebase Admin SDK's default, which asks for
// several Firebase products this codebase never touches (Phase 5 TD
// §9.2, keputusan M4, same reasoning as choosing net/smtp over a mail
// library in Phase 4.6).
const fcmMessagingScope = "https://www.googleapis.com/auth/firebase.messaging"

// ErrTokenInvalid is returned by FCMSender.Send when FCM itself reports
// the token can never succeed again — UNREGISTERED (the app was
// uninstalled) or INVALID_ARGUMENT (the token is malformed). Callers
// check this with errors.Is to decide whether to delete the token
// (Phase 5 TD §9.4, kriteria #12) — every other error (timeout, 5xx,
// network) is transient and must NOT delete anything.
var ErrTokenInvalid = errors.New("push: token invalid or unregistered")

// FCMConfig holds everything FCMSender needs to reach FCM. Every field
// maps directly to one PUSH_*/FCM_* env var (internal/shared/config).
type FCMConfig struct {
	ProjectID       string
	CredentialsFile string
	Timeout         time.Duration
}

// fcmBaseURL is overridden only from this package's own internal tests
// (fcm_internal_test.go), pointed at an httptest.Server standing in for
// FCM — there's no local FCM emulator equivalent to Mailpit (Phase 4.6)
// to run this against for real, so the HTTP behavior (request shape,
// status/error-body handling) is proven against a fake server instead,
// and the actual send is verified manually once Firebase exists
// (Phase 5 TD §12.1, issue #73).
const fcmBaseURL = "https://fcm.googleapis.com"

// FCMSender sends push notifications through FCM's HTTP v1 API
// directly — POST {baseURL}/v1/projects/{id}/messages:send — rather
// than the Firebase Admin SDK, which brings Firestore, Auth, and
// Storage clients this codebase never uses (Rule #27).
type FCMSender struct {
	projectID string
	ts        oauth2.TokenSource
	client    *http.Client
	baseURL   string
}

// NewFCMSender reads the service account file once at construction and
// builds a token source that refreshes itself as needed — the same
// long-lived object is reused across every Send call, never re-read
// from disk per request.
func NewFCMSender(cfg FCMConfig) (*FCMSender, error) {
	data, err := os.ReadFile(cfg.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("push: read credentials file: %w", err)
	}
	jwtCfg, err := google.JWTConfigFromJSON(data, fcmMessagingScope)
	if err != nil {
		return nil, fmt.Errorf("push: parse credentials file: %w", err)
	}
	return &FCMSender{
		projectID: cfg.ProjectID,
		ts:        jwtCfg.TokenSource(context.Background()),
		// Timeout bounds only the message-send request below — the
		// lesson from #63: smtp.SendMail had no timeout at all, and
		// Send here runs in the same best-effort, post-commit position
		// mailer.Send does. Set from the start, not discovered later.
		client:  &http.Client{Timeout: cfg.Timeout},
		baseURL: fcmBaseURL,
	}, nil
}

type fcmEnvelope struct {
	Message fcmMessage `json:"message"`
}

type fcmMessage struct {
	Token        string            `json:"token"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// fcmErrorBody is FCM v1's documented error shape. Only the one field
// this package acts on is decoded — errorCode inside details, which is
// where UNREGISTERED/INVALID_ARGUMENT actually appear (the top-level
// status string uses gRPC status names like NOT_FOUND, not FCM's own
// error codes).
type fcmErrorBody struct {
	Error struct {
		Details []struct {
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

func (s *FCMSender) Send(ctx context.Context, msg Message) error {
	tok, err := s.ts.Token()
	if err != nil {
		return fmt.Errorf("push: fetch oauth token: %w", err)
	}

	body, err := json.Marshal(fcmEnvelope{Message: fcmMessage{
		Token:        msg.Token,
		Notification: fcmNotification{Title: msg.Title, Body: msg.Body},
		Data:         msg.Data,
	}})
	if err != nil {
		return fmt.Errorf("push: marshal message: %w", err)
	}

	url := fmt.Sprintf("%s/v1/projects/%s/messages:send", s.baseURL, s.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("push: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("push: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if isUnregisteredError(resp.StatusCode, respBody) {
		return ErrTokenInvalid
	}
	return fmt.Errorf("push: fcm returned %d: %s", resp.StatusCode, respBody)
}

// isUnregisteredError matches the two codes TD §9.4 names — 404 with
// errorCode UNREGISTERED (app uninstalled) or 400 with INVALID_ARGUMENT
// (malformed token). Any other status (429, 5xx, other 400s) is left
// alone — those are transient or unrelated to the token itself, and
// deleting a token on a transient error would be a real user's push
// silently disabled by a momentary FCM hiccup.
func isUnregisteredError(statusCode int, body []byte) bool {
	if statusCode != http.StatusNotFound && statusCode != http.StatusBadRequest {
		return false
	}
	var parsed fcmErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	for _, d := range parsed.Error.Details {
		if d.ErrorCode == "UNREGISTERED" || d.ErrorCode == "INVALID_ARGUMENT" {
			return true
		}
	}
	return false
}
