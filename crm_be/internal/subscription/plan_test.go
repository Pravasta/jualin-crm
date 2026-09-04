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
