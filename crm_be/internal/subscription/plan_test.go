package subscription

import (
	"strings"
	"testing"
)

// --- "angka sudah diisi" (#126) ---

// TestLimitsAreNoLongerProvisional is #126's acceptance criterion
// verbatim: *"test 'angka sudah diisi' hijau — rilis dengan placeholder
// tidak mungkin terjadi diam-diam"*.
//
// It is the standing counterpart to cmd/api's production boot guard:
// the guard stops a provisional build from SERVING, this stops one from
// being called finished. Setting LimitsAreProvisional back to true for
// a future round of numbers turns this test red on purpose — that is
// the signal, not a nuisance to silence.
func TestLimitsAreNoLongerProvisional(t *testing.T) {
	if LimitsAreProvisional {
		t.Error("LimitsAreProvisional is still true: the product owner has not committed to these numbers, and cmd/api refuses to boot in production while that holds (ADR-014 ketentuan 1)")
	}
}

// TestPlanDisplay_NoPlaceholderPriceLabels covers the half of "angka
// sudah diisi" that planLimits does not: the quantities live in
// planLimits, but the price a customer actually reads lives here, and a
// forgotten placeholder ships a pricing screen that says nothing while
// every other test stays green.
func TestPlanDisplay_NoPlaceholderPriceLabels(t *testing.T) {
	// Exact matches, not substrings — "Negosiasi" (Enterprise, prd D4)
	// is a deliberate, final answer rather than a placeholder, and a
	// naive contains-check would have to special-case it.
	placeholders := map[string]bool{
		"segera": true, "tbd": true, "-": true, "?": true,
		"coming soon": true, "belum ditentukan": true, "(?)": true,
	}

	for code, display := range planDisplay {
		if display.Name == "" {
			t.Errorf("plan %q has no display name", code)
		}
		if display.PriceLabel == "" {
			t.Errorf("plan %q has no price label", code)
			continue
		}
		if placeholders[strings.ToLower(strings.TrimSpace(display.PriceLabel))] {
			t.Errorf("plan %q price label %q still reads as a placeholder — #126 requires a real answer", code, display.PriceLabel)
		}
	}
}

// TestChannelsFor_KnownPlan_MatchesMap is the table over the ENTIRE
// Channels set (TD §12) — not a hand-written list. Adding a channel to
// Channels without adding it to planChannels[PlanFree] fails this test
// automatically; that's the whole point of keeping Channels a slice
// rather than repeating it per test.
func TestChannelsFor_KnownPlan_MatchesMap(t *testing.T) {
	got := channelsFor(PlanFree, statusActive)

	if len(got) != len(Channels) {
		t.Fatalf("expected %d channels, got %d: %v", len(Channels), len(got), got)
	}
	for _, ch := range Channels {
		want, ok := planChannels[PlanFree][ch]
		if !ok {
			t.Fatalf("planChannels[%q] has no entry for %q — channel added to Channels without a policy decision", PlanFree, ch)
		}
		if got[ch] != want {
			t.Errorf("channel %q: got %v, want %v (per planChannels)", ch, got[ch], want)
		}
	}
}

func TestChannelsFor_UnknownPlan_AllClosed(t *testing.T) {
	got := channelsFor("nonexistent-plan", statusActive)

	if len(got) != len(Channels) {
		t.Fatalf("expected an entry for every channel, got %d: %v", len(got), got)
	}
	for _, ch := range Channels {
		if got[ch] {
			t.Errorf("channel %q: expected closed for unknown plan, got open", ch)
		}
	}
}

func TestParseChannel_KnownValues_RoundTrip(t *testing.T) {
	for _, ch := range Channels {
		got, err := ParseChannel(string(ch))
		if err != nil {
			t.Errorf("ParseChannel(%q): unexpected error: %v", ch, err)
		}
		if got != ch {
			t.Errorf("ParseChannel(%q) = %q, want %q", ch, got, ch)
		}
	}
}

func TestParseChannel_UnknownValue_ReturnsError(t *testing.T) {
	if _, err := ParseChannel("apikey"); err == nil {
		t.Error("expected an error for a near-miss typo, got nil")
	}
	if _, err := ParseChannel(""); err == nil {
		t.Error("expected an error for empty string, got nil")
	}
}

func TestChannelsFor_NonActiveStatus_AllClosedRegardlessOfPlan(t *testing.T) {
	for _, status := range []string{"past_due", "suspended", "canceled", ""} {
		got := channelsFor(PlanFree, status)
		for _, ch := range Channels {
			if got[ch] {
				t.Errorf("status %q: channel %q expected closed (only 'active' opens anything), got open", status, ch)
			}
		}
	}
}

// --- Limits (Phase 8.5) ---

// TestPlanLimits_EveryPlanHasEntry is the table over EVERY plan in
// planChannels — not a hand-written list of three. A plan added to one
// map and forgotten in the other fails here rather than silently
// resolving to free-tier quantities in production.
func TestPlanLimits_EveryPlanHasEntry(t *testing.T) {
	for plan := range planChannels {
		if _, ok := planLimits[plan]; !ok {
			t.Errorf("plan %q is in planChannels but missing from planLimits", plan)
		}
	}
	for plan := range planLimits {
		if _, ok := planChannels[plan]; !ok {
			t.Errorf("plan %q is in planLimits but missing from planChannels", plan)
		}
	}
}

// TestPlanCatalog_AllFourCollectionsAgree extends the check above to
// planDisplay and planOrder (added in #125) — a plan missing from any
// ONE of the four leaves either a silent free-tier fallback (planLimits/
// planChannels, already covered above) or a plan the comparison screen
// can never show/never renders a name for (planOrder/planDisplay).
func TestPlanCatalog_AllFourCollectionsAgree(t *testing.T) {
	inOrder := make(map[string]bool, len(planOrder))
	for _, code := range planOrder {
		inOrder[code] = true
	}

	for plan := range planChannels {
		if !inOrder[plan] {
			t.Errorf("plan %q is in planChannels but missing from planOrder", plan)
		}
		if _, ok := planDisplay[plan]; !ok {
			t.Errorf("plan %q is in planChannels but missing from planDisplay", plan)
		}
	}
	for plan := range inOrder {
		if _, ok := planChannels[plan]; !ok {
			t.Errorf("plan %q is in planOrder but missing from planChannels", plan)
		}
	}
	for plan := range planDisplay {
		if _, ok := planChannels[plan]; !ok {
			t.Errorf("plan %q is in planDisplay but missing from planChannels", plan)
		}
	}
	if len(planOrder) != len(inOrder) {
		t.Errorf("planOrder has a duplicate entry: %v", planOrder)
	}
}

// TestCatalog_ReturnsEveryPlanInOrder locks the display order the
// dashboard's three columns render in — Free, Pro, Enterprise — since a
// map (planChannels/planLimits/planDisplay) has none of its own.
func TestCatalog_ReturnsEveryPlanInOrder(t *testing.T) {
	got := Catalog()

	if len(got) != len(planOrder) {
		t.Fatalf("expected %d plans, got %d", len(planOrder), len(got))
	}
	for i, code := range planOrder {
		if got[i].Code != code {
			t.Errorf("position %d: got %q, want %q", i, got[i].Code, code)
		}
	}
}

// TestCatalog_EachEntryMatchesItsSourceMaps proves Catalog() is a pure
// read of the four maps above, not a second copy of their numbers —
// changing planLimits/planChannels/planDisplay and not Catalog() would
// otherwise be invisible until the comparison screen shows stale data.
func TestCatalog_EachEntryMatchesItsSourceMaps(t *testing.T) {
	for _, entry := range Catalog() {
		if entry.Limits != planLimits[entry.Code] {
			t.Errorf("%q: Limits %+v does not match planLimits", entry.Code, entry.Limits)
		}
		if entry.Name != planDisplay[entry.Code].Name {
			t.Errorf("%q: Name %q does not match planDisplay", entry.Code, entry.Name)
		}
		if entry.PriceLabel != planDisplay[entry.Code].PriceLabel {
			t.Errorf("%q: PriceLabel %q does not match planDisplay", entry.Code, entry.PriceLabel)
		}
		for _, ch := range Channels {
			if entry.Channels[ch] != planChannels[entry.Code][ch] {
				t.Errorf("%q channel %q: got %v, want %v", entry.Code, ch, entry.Channels[ch], planChannels[entry.Code][ch])
			}
		}
	}
}

// TestCatalog_ResolvesEveryPlanAsIfActive proves the catalog describes
// what a plan OFFERS, not any one organization's current status — every
// entry must be resolved as "active", even though no organization in
// the database is guaranteed to hold that status for that plan right
// now.
func TestCatalog_ResolvesEveryPlanAsIfActive(t *testing.T) {
	for _, entry := range Catalog() {
		want := channelsFor(entry.Code, statusActive)
		for _, ch := range Channels {
			if entry.Channels[ch] != want[ch] {
				t.Errorf("%q channel %q: got %v, want %v (resolved as active)", entry.Code, ch, entry.Channels[ch], want[ch])
			}
		}
	}
}

// TestLimitsFor_UnknownPlan_FallsBackToFreeNotZero locks the deliberate
// asymmetry with channelsFor (TD 8.5 §2.1): channels close completely,
// quantities drop to the strictest KNOWN plan. Zero here would stop the
// product accepting leads at all, which a billing failure must never do.
func TestLimitsFor_UnknownPlan_FallsBackToFreeNotZero(t *testing.T) {
	got := limitsFor("nonexistent-plan", statusActive)

	if got != planLimits[PlanFree] {
		t.Fatalf("expected free-tier limits for an unknown plan, got %+v", got)
	}
	if got.LeadsPerMonth == Unlimited {
		t.Error("unknown plan resolved to an UNLIMITED lead quota — fails open, the one thing this must never do")
	}
}

func TestLimitsFor_NonActiveStatus_FallsBackToFree(t *testing.T) {
	for _, status := range []string{"past_due", "suspended", "canceled", ""} {
		got := limitsFor(PlanPro, status)
		if got != planLimits[PlanFree] {
			t.Errorf("status %q: expected free-tier limits, got %+v", status, got)
		}
	}
}

func TestLimitsFor_KnownActivePlan_ReturnsItsOwnLimits(t *testing.T) {
	for plan, want := range planLimits {
		if got := limitsFor(plan, statusActive); got != want {
			t.Errorf("plan %q: got %+v, want %+v", plan, got, want)
		}
	}
}

// TestAllows_ZeroMeansUnlimited is the one place the meaning of 0 is
// asserted. Every other call site asks allows() precisely so this
// meaning lives in a single line of production code.
func TestAllows_ZeroMeansUnlimited(t *testing.T) {
	if !allows(Unlimited, 1_000_000) {
		t.Error("Unlimited (0) must allow any usage — it means unlimited, not none")
	}
}

func TestAllows_BelowAtAndAboveLimit(t *testing.T) {
	cases := []struct {
		limit, used int
		want        bool
	}{
		{limit: 2, used: 0, want: true},
		{limit: 2, used: 1, want: true},
		{limit: 2, used: 2, want: false}, // at the limit: the next one does not fit
		{limit: 2, used: 3, want: false}, // over (possible under concurrency, prd D2)
	}
	for _, c := range cases {
		if got := allows(c.limit, c.used); got != c.want {
			t.Errorf("allows(limit=%d, used=%d) = %v, want %v", c.limit, c.used, got, c.want)
		}
	}
}

// TestEnterpriseIsUnlimited pins the one plan whose numbers are NOT
// provisional — "enterprise negotiates" means no quota to guess.
func TestEnterpriseIsUnlimited(t *testing.T) {
	l := planLimits[PlanEnterprise]
	if l.LeadsPerMonth != Unlimited || l.Seats != Unlimited {
		t.Errorf("expected enterprise to be unlimited on both meters, got %+v", l)
	}
}
