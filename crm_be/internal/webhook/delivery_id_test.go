package webhook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// The splice exists because the body we sign must be byte-identical to the
// body we send (docs/issues/101). These tests hold both halves of that:
// the id really lands in the JSON, and nothing else about the bytes moves.

func TestInjectDeliveryID_AddsFieldAndKeepsTheRestByteIdentical(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	payload := []byte(`{"event":"lead.created","occurred_at":"2026-09-02T10:00:00Z","data":{"lead":{"name":"Budi"}}}`)

	got := injectDeliveryID(payload, id)

	// Valid JSON, with the id present.
	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, got)
	}
	if decoded["delivery_id"] != id.String() {
		t.Errorf("delivery_id = %v, want %s", decoded["delivery_id"], id)
	}

	// Everything the original said, still said the same way. Comparing the
	// tail byte-for-byte is the point: a re-marshal would reorder keys and
	// break every signature without changing the decoded value at all.
	tail := string(got[len(`{"delivery_id":"`+id.String()+`",`):])
	if want := string(payload[1:]); tail != want {
		t.Errorf("the original bytes were rewritten\n got %q\nwant %q", tail, want)
	}
}

func TestInjectDeliveryID_EmptyObject(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	got := injectDeliveryID([]byte(`{}`), id)

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("result is not valid JSON: %v (%s)", err, got)
	}
	if len(decoded) != 1 || decoded["delivery_id"] != id.String() {
		t.Errorf("got %s, want an object holding only delivery_id", got)
	}
	if strings.Contains(string(got), ",}") {
		t.Errorf("trailing comma left behind: %s", got)
	}
}

// TestInjectDeliveryID_NonObjectIsLeftAlone documents the defensive
// choice: a payload that is not an object cannot be spliced, and a
// delivery missing delivery_id is a degraded one, while a delivery with a
// stray brace is an unparseable one. Enqueue can only ever store a
// marshalled struct, so this is a guard rather than a live path.
func TestInjectDeliveryID_NonObjectIsLeftAlone(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	for _, payload := range []string{``, `[]`, `"just a string"`, `null`, `{`} {
		if got := injectDeliveryID([]byte(payload), id); string(got) != payload {
			t.Errorf("injectDeliveryID(%q) = %q, want it returned untouched", payload, got)
		}
	}
}

func TestInjectDeliveryID_IsFirstKey(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	got := injectDeliveryID([]byte(`{"event":"lead.created"}`), id)

	if !strings.HasPrefix(string(got), `{"delivery_id":"`+id.String()+`",`) {
		t.Errorf("delivery_id is not the first key: %s", got)
	}
}
