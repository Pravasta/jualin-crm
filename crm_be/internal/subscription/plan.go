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

// The three plans (Phase 8.5). Still plain `text` in plan_code, with no
// plans table — prd D2 of Phase 8 holds: one column is enough until
// there is a real second implementation, and three literals is not it.
const (
	PlanFree       = "free"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)

// planChannels is THE map (prd D3). Opening or closing a channel for a
// plan is a one-line change here and nowhere else (kriteria #2).
//
// Every channel is open on `free` today. That is not a placeholder: it
// is the honest state of the product until pricing exists (ADR-012 §4,
// prd kriteria #9). The mechanism is what ships now; the numbers
// replace this literal later (TD §16).
// Every channel is open on every plan TODAY, including free. That is
// deliberate and not an oversight: closing a channel that free already
// opens would take something away from organizations already using it,
// which is the downgrade behaviour Phase 8 D4 refused to build. Which
// channels each paid plan actually distinguishes is part of the
// provisional-numbers decision still owed by the product owner
// (08.5-paid-plans/prd.md, tabel *Angka provisional*) — until then the
// only real differentiator is planLimits below.
var planChannels = map[string]map[Channel]bool{
	PlanFree:       {ChannelAPIKey: true, ChannelForm: true, ChannelWebhook: true},
	PlanPro:        {ChannelAPIKey: true, ChannelForm: true, ChannelWebhook: true},
	PlanEnterprise: {ChannelAPIKey: true, ChannelForm: true, ChannelWebhook: true},
}

// Unlimited is what a zero means inside Limits. It is NOT "none" — the
// distinction matters enough that no call site compares against 0
// directly; allows() below is the only place that knows.
const Unlimited = 0

// Limits is what a plan allows in QUANTITY — the dimension Phase 8
// deliberately did not have (TD 8.5 §2). Kept beside planChannels
// rather than merged into it because "which channel" and "how many" are
// different questions with different failure modes (§2.1).
type Limits struct {
	LeadsPerMonth int
	Seats         int
}

// LimitsAreProvisional guards the one thing that must never ship by
// accident: numbers nobody chose. The composition root refuses to boot
// with APP_ENV=production while this is true (cmd/api, ADR-010's
// fail-fast) — the same shape as WEBHOOK_ALLOW_PRIVATE_TARGETS (#100)
// and CAPTCHA_PROVIDER=none (#87).
//
// TD 8.5 §14 originally proposed a deliberately failing test for this.
// A red test in CI trains people to ignore red; a boot that refuses
// production cannot be ignored and cannot be forgotten. Flipped to
// false in the closing issue (#126), together with the real numbers.
const LimitsAreProvisional = true

// planLimits is THE map for quantities. Adding a plan or changing a
// quota is a one-line change here and nowhere else.
//
// ⚠️ ANGKA DI BAWAH PROVISIONAL — dipilih tanpa data pengguna nyata
// (ADR-014). Wajib ditinjau ulang setelah 3–5 pelanggan berbayar
// pertama, keempatnya bersama-sama: kuota Free, kuota Pro, harga Pro,
// batas seat.
var planLimits = map[string]Limits{
	PlanFree:       {LeadsPerMonth: 100, Seats: 2},
	PlanPro:        {LeadsPerMonth: 2000, Seats: 10},
	PlanEnterprise: {LeadsPerMonth: Unlimited, Seats: Unlimited},
}

// planOrder is the plan catalog's display order (Free, Pro, Enterprise)
// — a product decision, not something a map (which has none) can carry.
// Kept beside planChannels/planLimits/planDisplay rather than in the
// dashboard: kriteria #6/#9 (08.5-paid-plans) forbid a second,
// TypeScript-side copy of anything about what a plan offers, ordering
// included.
var planOrder = []string{PlanFree, PlanPro, PlanEnterprise}

// PlanDisplay is a plan's human-facing name and price label — the ONLY
// place price text lives (prd 8.5 D7: "satu peta Go + satu tabel
// dokumen", never TypeScript).
//
// ⚠️ PriceLabel below is a placeholder, not a real price: Pro's number
// is still "(?)" in prd 8.5's angka provisional table, and Enterprise
// is a negotiated conversation, never a checkout (prd D4). Both are
// worded so they read honestly on the pricing screen rather than as a
// number nobody chose — the LimitsAreProvisional boot guard already
// stops this state from reaching a production deploy silently, but a
// vague label is still safer than a plausible-looking fake number.
type PlanDisplay struct {
	Name       string
	PriceLabel string
}

var planDisplay = map[string]PlanDisplay{
	PlanFree:       {Name: "Free", PriceLabel: "Rp0"},
	PlanPro:        {Name: "Pro", PriceLabel: "Segera"},
	PlanEnterprise: {Name: "Enterprise", PriceLabel: "Negosiasi"},
}

// PlanCatalogEntry is one row of the plan comparison — everything
// GET /v1/plans sends for one plan, already fully resolved server-side
// (Phase 8 kriteria #6, prd 8.5 kriteria #9: the dashboard renders this
// exactly, computing nothing of its own about what a plan offers).
type PlanCatalogEntry struct {
	Code       string
	Name       string
	PriceLabel string
	Limits     Limits
	Channels   map[Channel]bool
}

// Catalog returns every plan in product display order, each resolved as
// though its subscription were active — this describes what a plan
// OFFERS, independent of any one organization's current status (that
// question belongs to ResolvePlan, not here).
func Catalog() []PlanCatalogEntry {
	out := make([]PlanCatalogEntry, 0, len(planOrder))
	for _, code := range planOrder {
		out = append(out, PlanCatalogEntry{
			Code:       code,
			Name:       planDisplay[code].Name,
			PriceLabel: planDisplay[code].PriceLabel,
			Limits:     planLimits[code],
			Channels:   channelsFor(code, statusActive),
		})
	}
	return out
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

// limitsFor resolves the quantities planCode allows, gated by status.
//
// It fails closed to the STRICTEST KNOWN PLAN (free), NOT to zero — and
// that is a deliberate asymmetry with channelsFor above, which does
// close everything. The two have different blast radii (TD 8.5 §2.1):
//
//   - A closed channel means "you cannot create a NEW api key / form /
//     webhook". Annoying; nothing already running stops.
//   - A zero lead quota would mean the product stops accepting leads
//     AT ALL — the customer's embedded form dies, their integration
//     dies. A billing hiccup must never delete the core function.
//
// So an unknown plan_code, or a subscription that is not active, drops
// the organization to free-tier quantities until the situation is
// sorted out. That is the standard SaaS behaviour and it is reversible.
func limitsFor(planCode, status string) Limits {
	if status != statusActive {
		return planLimits[PlanFree]
	}
	l, ok := planLimits[planCode]
	if !ok {
		return planLimits[PlanFree]
	}
	return l
}

// allows reports whether one more unit fits under limit, given how many
// are already used. Unlimited (0) always fits.
//
// The ONLY place that knows what 0 means. Callers ask this question
// rather than comparing numbers themselves, so "0 is unlimited, not
// none" lives in exactly one line of the codebase.
func allows(limit, used int) bool {
	if limit == Unlimited {
		return true
	}
	return used < limit
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
