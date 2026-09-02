package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// SignatureHeader carries the proof that a delivery came from us. Named
// with our own prefix rather than a generic X-Signature so a receiver
// fronting several vendors can tell whose scheme to apply.
const SignatureHeader = "X-Jualin-Signature"

// Sign returns the SignatureHeader value for one delivery attempt:
//
//	t=<unix seconds>,v1=<hex HMAC-SHA256>
//
// over the signed payload "<unix seconds>.<body>", with body used as the
// exact bytes that go on the wire — never re-marshalled, since any
// re-encoding (key order, whitespace, escaping) yields a different
// signature than what the receiver will verify against.
//
// The timestamp is INSIDE the signed payload, not merely alongside it.
// That is the whole point of the t=/v1= shape: if t sat outside, anyone
// who captured one valid request could replay it forever by rewriting t
// to the current time, because the signature would only ever have covered
// the body. Binding them together makes editing t invalidate v1, which
// is what lets a receiver reject stale deliveries at all. This is the
// single most commonly botched detail in hand-rolled webhook signing, so
// signature_test.go asserts it directly rather than trusting the shape.
//
// The 5-minute tolerance in the receiver documentation is the receiver's
// to enforce; we neither impose nor assume it here. A retry hours later
// is signed with the time it is actually sent, not the time it was
// enqueued — otherwise every retry would arrive already outside the
// window it is supposed to be judged by.
//
// v1 names the scheme, not the version of any one delivery. A future v2
// would be sent ALONGSIDE v1 in the same header so receivers can migrate
// without a flag day; nothing here needs to change for that to be
// possible, which is why the prefix exists now while there is only one.
func Sign(secret string, ts time.Time, body []byte) string {
	unix := strconv.FormatInt(ts.Unix(), 10)

	mac := hmac.New(sha256.New, []byte(secret))
	// hash.Hash's Write never returns an error (documented on the
	// interface), so the errcheck-silencing assignment other Write call
	// sites need is not warranted here.
	mac.Write([]byte(unix))
	mac.Write([]byte("."))
	mac.Write(body)

	return "t=" + unix + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}
