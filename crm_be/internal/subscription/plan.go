package subscription

import "fmt"

// Channel identifies one of the capabilities a plan can open or close.
// String-typed (not int) because its literal values are a wire contract
// with internal/auth's GET /v1/me response and with three other
// packages' PlanGate interfaces (TD §7) — a typo here must be a compile
// error, not a silent mismatch.
type Channel string

const (
	ChannelAPIKey  Channel = "api_key"
	ChannelForm    Channel = "form"
	ChannelWebhook Channel = "webhook"
)

// Channels is the closed set every table-driven test iterates over — a
// hand-written list per test is what lets a new channel be forgotten
// (TD §2, §12). Adding a fourth channel means adding one constant above
// and one entry here; every test keyed off this slice picks it up
// automatically.
var Channels = []Channel{ChannelAPIKey, ChannelForm, ChannelWebhook}

// PlanFree is the only plan that exists today (prd D2 — no plans
// table).
const PlanFree = "free"

// planChannels is THE map (prd D3). Opening or closing a channel for a
// plan is a one-line change here and nowhere else (kriteria #2).
//
// Every channel is open on `free` today. That is not a placeholder: it
// is the honest state of the product until pricing exists (ADR-012 §4,
// prd kriteria #9). The mechanism is what ships now; the numbers
// replace this literal later (TD §16).
var planChannels = map[string]map[Channel]bool{
	PlanFree: {ChannelAPIKey: true, ChannelForm: true, ChannelWebhook: true},
}

const statusActive = "active"

// channelsFor resolves which channels planCode opens, gated by status.
// It always returns an entry for every member of Channels — never a
// partial map — so callers never need a second "was this key even
// present" check.
//
// Fails CLOSED, not open (TD §2, kriteria #8):
//   - status != "active" closes every channel, regardless of planCode
//     (TD §1.1) — past_due/suspended/canceled all deny.
//   - a planCode absent from planChannels closes every channel. plan_code
//     is `text` without a CHECK constraint; an unexpected value can
//     arrive from outside the CRM (the payment service writes it), and
//     the default for anything unrecognized must be deny.
func channelsFor(planCode, status string) map[Channel]bool {
	out := make(map[Channel]bool, len(Channels))

	open := status == statusActive
	plan := planChannels[planCode]
	for _, ch := range Channels {
		out[ch] = open && plan[ch]
	}
	return out
}

// ParseChannel validates s against Channels and returns the typed
// Channel. Every PlanGate implementation (§3.2: apikey/form/webhook each
// declare their own, keyed by a plain string so they never import this
// package) calls this at the composition-root bridge rather than
// converting blindly with Channel(s).
//
// The distinction matters precisely because of the risk TD §7 names: a
// typo in one of the four places the "api_key"/"form"/"webhook" literals
// are duplicated makes that channel silently ALWAYS closed — planChannels
// has no entry for the misspelled value, and channelsFor's fail-closed
// behavior makes that indistinguishable from an honest deny. A blind
// conversion ships that bug as a normal-looking 403. ParseChannel turns
// it into a loud error on the very first request instead.
func ParseChannel(s string) (Channel, error) {
	for _, ch := range Channels {
		if string(ch) == s {
			return ch, nil
		}
	}
	return "", fmt.Errorf("subscription: unknown channel %q", s)
}
