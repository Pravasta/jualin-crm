package webhook

import (
	"bytes"

	"github.com/google/uuid"
)

// injectDeliveryID adds "delivery_id" as the first key of the stored
// event snapshot, returning the bytes that are both signed and sent.
//
// Why it is spliced rather than unmarshalled and re-marshalled: the
// receiver verifies the signature over the exact bytes it received, so the
// body we sign has to be byte-identical to the body we send. Round-tripping
// through map[string]any would reorder keys and re-escape strings, which
// changes nothing semantically and breaks every signature — silently, with
// no error on our side (the wire contract in docs/issues/101).
//
// Why the id is not in the snapshot to begin with: one snapshot is shared
// by every endpoint subscribed to the event, while delivery_id differs per
// row. Each row already carries it as its own primary key, stable across
// every retry of that row (TD §4.2), which is exactly the deduplication
// handle an at-least-once receiver needs.
//
// The splice is safe for anything Enqueue can hold — payload always comes
// from json.Marshal of a struct, so it is a compact object starting with
// '{'. Anything else is returned untouched rather than corrupted: a body
// without delivery_id is a degraded delivery, a body with a stray brace is
// an unparseable one.
func injectDeliveryID(payload []byte, id uuid.UUID) []byte {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) < 2 || trimmed[0] != '{' {
		return payload
	}

	field := `"delivery_id":"` + id.String() + `"`

	out := make([]byte, 0, len(trimmed)+len(field)+1)
	out = append(out, '{')
	out = append(out, field...)
	// An object with no other keys takes no separator; anything else does.
	if bytes.TrimSpace(trimmed[1:])[0] != '}' {
		out = append(out, ',')
	}
	out = append(out, trimmed[1:]...)
	return out
}
