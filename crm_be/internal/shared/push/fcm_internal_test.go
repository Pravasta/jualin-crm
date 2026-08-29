package push

// package push (not push_test) — FCMSender's baseURL field is
// unexported by design (only ever overridden from here, pointed at an
// httptest.Server standing in for FCM — Phase 5 TD §9.2's doc comment
// on fcmBaseURL explains why there's no real FCM emulator to test
// against the way Mailpit lets Phase 4.6 test SMTP for real). Mirrors
// the same isolated exception mailer's message_internal_test.go and
// ratelimit's limiter_internal_test.go already set precedent for.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func fakeTokenSource() oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake-token", Expiry: time.Now().Add(time.Hour)})
}

func newTestFCMSender(t *testing.T, serverURL string) *FCMSender {
	t.Helper()
	return &FCMSender{
		projectID: "test-project",
		ts:        fakeTokenSource(),
		client:    &http.Client{Timeout: 5 * time.Second},
		baseURL:   serverURL,
	}
}

func TestFCMSender_Send_SendsCorrectRequestShape(t *testing.T) {
	var gotAuth, gotPath, gotMethod string
	var gotBody struct {
		Message struct {
			Token        string `json:"token"`
			Notification struct{ Title, Body string }
			Data         map[string]string `json:"data"`
		} `json:"message"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"projects/test-project/messages/0"}`))
	}))
	defer srv.Close()

	s := newTestFCMSender(t, srv.URL)
	err := s.Send(context.Background(), Message{
		Token: "device-token-abc",
		Title: "Lead #42 ditugaskan kepada Anda",
		Body:  "Budi Santoso",
		Data:  map[string]string{"type": "lead_assigned", "lead_id": "01a0"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/v1/projects/test-project/messages:send" {
		t.Errorf("unexpected path: %s", gotPath)
	}
	if gotAuth != "Bearer fake-token" {
		t.Errorf("expected Authorization: Bearer fake-token, got %q", gotAuth)
	}
	if gotBody.Message.Token != "device-token-abc" {
		t.Errorf("expected token in body, got %q", gotBody.Message.Token)
	}
	if gotBody.Message.Data["lead_id"] != "01a0" {
		t.Errorf("expected data.lead_id=01a0, got %+v", gotBody.Message.Data)
	}
}

// TestFCMSender_Send_UnregisteredTokenReturnsErrTokenInvalid is Phase 5
// TD §9.4's direct acceptance criterion: 404 UNREGISTERED must be
// distinguishable from every other failure so the caller (device.Usecase)
// knows to delete the token.
func TestFCMSender_Send_UnregisteredTokenReturnsErrTokenInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Requested entity was not found.","status":"NOT_FOUND","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`))
	}))
	defer srv.Close()

	s := newTestFCMSender(t, srv.URL)
	err := s.Send(context.Background(), Message{Token: "dead-token", Title: "t", Body: "b"})
	if err != ErrTokenInvalid {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

// TestFCMSender_Send_InvalidArgumentReturnsErrTokenInvalid covers the
// second case TD §9.4 names — a malformed token, not just an
// uninstalled app.
func TestFCMSender_Send_InvalidArgumentReturnsErrTokenInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"The registration token is not a valid FCM registration token","status":"INVALID_ARGUMENT","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"INVALID_ARGUMENT"}]}}`))
	}))
	defer srv.Close()

	s := newTestFCMSender(t, srv.URL)
	err := s.Send(context.Background(), Message{Token: "malformed-token", Title: "t", Body: "b"})
	if err != ErrTokenInvalid {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

// TestFCMSender_Send_TransientErrorDoesNotReturnErrTokenInvalid proves
// isUnregisteredError doesn't over-match — a rate limit or server error
// must NOT look like "delete this token", or a momentary FCM hiccup
// would silently disable a real user's push forever.
func TestFCMSender_Send_TransientErrorDoesNotReturnErrTokenInvalid(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"429 rate limited", http.StatusTooManyRequests, `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`},
		{"500 internal", http.StatusInternalServerError, `{"error":{"code":500,"status":"INTERNAL"}}`},
		{"400 other reason", http.StatusBadRequest, `{"error":{"code":400,"status":"INVALID_ARGUMENT","details":[{"errorCode":"THIRD_PARTY_AUTH_ERROR"}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			s := newTestFCMSender(t, srv.URL)
			err := s.Send(context.Background(), Message{Token: "some-token", Title: "t", Body: "b"})
			if err == nil {
				t.Fatal("expected an error")
			}
			if err == ErrTokenInvalid {
				t.Errorf("expected a transient error, got ErrTokenInvalid — this would wrongly delete a live token")
			}
		})
	}
}

// TestFCMSender_Send_RespectsTimeout is the same lesson #63 already
// proved for SMTPMailer, applied here: a server that accepts the
// connection and never responds must not hang Send forever.
func TestFCMSender_Send_RespectsTimeout(t *testing.T) {
	// A fixed sleep well past the client's 300ms timeout, not
	// <-r.Context().Done(): whether the server notices a client giving
	// up depends on OS/connection details that aren't reliable in a
	// test, and httptest.Server.Close() then hangs waiting for a
	// handler that may never see its context canceled. Sleeping
	// guarantees the handler finishes on its own, on a schedule the
	// client has already stopped waiting for.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &FCMSender{
		projectID: "test-project",
		ts:        fakeTokenSource(),
		client:    &http.Client{Timeout: 300 * time.Millisecond},
		baseURL:   srv.URL,
	}

	start := time.Now()
	err := s.Send(context.Background(), Message{Token: "t", Title: "t", Body: "b"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Send to fail against a server that never responds")
	}
	if elapsed > 5*time.Second {
		t.Errorf("expected Send to fail within a bounded time close to the configured 300ms timeout, took %s", elapsed)
	}
}
