package subscription

import "testing"

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
