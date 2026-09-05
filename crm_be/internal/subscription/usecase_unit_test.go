package subscription_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
)

// fakeRepo lets tests control exactly what FindActiveByOrg returns
// without touching Postgres — TestUnit_* naming mirrors internal/auth's
// convention for fake-Store tests (ADR-011: business logic here doesn't
// touch pgx, so it shouldn't need Docker to be tested).
type fakeRepo struct {
	sub           *subscription.Subscription
	err           error
	changedTo     string
	changePlanErr error
}

func (f *fakeRepo) FindActiveByOrg(_ context.Context, _ tenant.Context) (*subscription.Subscription, error) {
	return f.sub, f.err
}

// ChangePlan exists so *fakeRepo still satisfies subscription.Repository
// (#124) — records what it was asked to change to, and lets a test
// inject a failure.
func (f *fakeRepo) ChangePlan(_ context.Context, _ tenant.Context, planCode string) error {
	if f.changePlanErr != nil {
		return f.changePlanErr
	}
	f.changedTo = planCode
	return nil
}

func testTenant() tenant.Context {
	return tenant.Context{PrincipalType: tenant.PrincipalUser}
}

func TestUnit_RequireChannel_OpenChannel_ReturnsNil(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}})

	if err := u.RequireChannel(context.Background(), testTenant(), subscription.ChannelWebhook); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestUnit_RequireChannel_ClosedChannel_Returns403PlanUpgradeRequired(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: "nonexistent-plan", Status: "active"}})

	err := u.RequireChannel(context.Background(), testTenant(), subscription.ChannelWebhook)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) {
		t.Fatalf("expected *httpx.DomainError, got %v (%T)", err, err)
	}
	if derr.Status != 403 || derr.Code != "plan_upgrade_required" {
		t.Errorf("expected 403 plan_upgrade_required, got %d %s", derr.Status, derr.Code)
	}
}

func TestUnit_RequireChannel_NoActiveSubscription_Returns403PlanUpgradeRequired(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{err: subscription.ErrNoActiveSubscription})

	err := u.RequireChannel(context.Background(), testTenant(), subscription.ChannelAPIKey)

	var derr *httpx.DomainError
	if !errors.As(err, &derr) {
		t.Fatalf("expected *httpx.DomainError, got %v (%T)", err, err)
	}
	if derr.Code != "plan_upgrade_required" {
		t.Errorf("expected plan_upgrade_required, got %s", derr.Code)
	}
}

func TestUnit_ResolvePlan_KeySetMatchesChannels(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}})

	code, channels, limits, err := u.ResolvePlan(context.Background(), testTenant())
	if err != nil {
		t.Fatalf("resolve plan: %v", err)
	}
	if limits.LeadsPerMonth <= 0 || limits.Seats <= 0 {
		t.Errorf("free plan must carry real quantities, got %+v", limits)
	}
	if code != subscription.PlanFree {
		t.Errorf("expected code %q, got %q", subscription.PlanFree, code)
	}
	if len(channels) != len(subscription.Channels) {
		t.Fatalf("expected %d keys, got %d: %v", len(subscription.Channels), len(channels), channels)
	}
	for _, ch := range subscription.Channels {
		if _, ok := channels[string(ch)]; !ok {
			t.Errorf("channels missing key %q — this is the wire contract TD §7 locks", ch)
		}
	}
}

func TestUnit_ResolvePlan_NoActiveSubscription_AllChannelsClosedNoError(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{err: subscription.ErrNoActiveSubscription})

	code, channels, limits, err := u.ResolvePlan(context.Background(), testTenant())
	if err != nil {
		t.Fatalf("expected no error (fail closed, not fail loud), got %v", err)
	}
	// Channels close completely, quantities drop to the free tier —
	// the deliberate asymmetry in TD 8.5 §2.1. Zero leads would mean the
	// product stops accepting leads at all.
	if limits.LeadsPerMonth <= 0 {
		t.Errorf("expected free-tier lead quota when there is no active subscription, got %d", limits.LeadsPerMonth)
	}
	if code != "" {
		t.Errorf("expected empty code, got %q", code)
	}
	for ch, ok := range channels {
		if ok {
			t.Errorf("channel %q: expected closed when there is no active subscription", ch)
		}
	}
}

// --- RequireLeadQuota (Phase 8.5 #123) ---

func TestUnit_RequireLeadQuota_UnderLimit_ReturnsNil(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}})

	if err := u.RequireLeadQuota(context.Background(), testTenant(), 0); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestUnit_RequireLeadQuota_AtLimit_Returns403PlanQuotaExceeded(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}})

	// The free plan's own configured limit — read from the map itself
	// rather than hardcoded, so this test does not need updating every
	// time the provisional number changes.
	err := u.RequireLeadQuota(context.Background(), testTenant(), quotaTestFreeLeadLimit(t))

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Status != 403 || derr.Code != "plan_quota_exceeded" {
		t.Fatalf("expected 403 plan_quota_exceeded, got %v", err)
	}
}

func TestUnit_RequireLeadQuota_NoActiveSubscription_FallsBackToFreeLimit(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{err: subscription.ErrNoActiveSubscription})

	// Well under any plausible limit — proves the fallback still allows
	// SOME usage (free-tier limits, not zero), per TD 8.5 §2.1.
	if err := u.RequireLeadQuota(context.Background(), testTenant(), 0); err != nil {
		t.Fatalf("expected the free-tier fallback to allow usage 0, got %v", err)
	}
}

// quotaTestFreeLeadLimit reads subscription.PlanFree's own configured
// LeadsPerMonth via ResolvePlan, rather than hardcoding the provisional
// number in this test file a second time.
func quotaTestFreeLeadLimit(t *testing.T) int {
	t.Helper()
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}})
	_, _, limits, err := u.ResolvePlan(context.Background(), testTenant())
	if err != nil {
		t.Fatalf("resolve plan: %v", err)
	}
	return limits.LeadsPerMonth
}

// --- RequireSeatLimit (Phase 8.5 #124) ---

func TestUnit_RequireSeatLimit_UnderLimit_ReturnsNil(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}})

	if err := u.RequireSeatLimit(context.Background(), testTenant(), 0); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestUnit_RequireSeatLimit_AtLimit_Returns403PlanSeatLimitReached(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}})

	err := u.RequireSeatLimit(context.Background(), testTenant(), quotaTestFreeSeatLimit(t))

	var derr *httpx.DomainError
	if !errors.As(err, &derr) || derr.Status != 403 || derr.Code != "plan_seat_limit_reached" {
		t.Fatalf("expected 403 plan_seat_limit_reached, got %v", err)
	}
}

func TestUnit_RequireSeatLimit_NoActiveSubscription_FallsBackToFreeLimit(t *testing.T) {
	u := subscription.NewUsecase(&fakeRepo{err: subscription.ErrNoActiveSubscription})

	if err := u.RequireSeatLimit(context.Background(), testTenant(), 0); err != nil {
		t.Fatalf("expected the free-tier fallback to allow usage 0, got %v", err)
	}
}

func quotaTestFreeSeatLimit(t *testing.T) int {
	t.Helper()
	u := subscription.NewUsecase(&fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}})
	_, _, limits, err := u.ResolvePlan(context.Background(), testTenant())
	if err != nil {
		t.Fatalf("resolve plan: %v", err)
	}
	return limits.Seats
}

// --- AdminChangePlan (Phase 8.5 #124) ---

func TestUnit_AdminChangePlan_KnownPlan_Succeeds(t *testing.T) {
	repo := &fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}}
	u := subscription.NewUsecase(repo)

	previous, err := u.AdminChangePlan(context.Background(), testTenant(), subscription.PlanPro)
	if err != nil {
		t.Fatalf("admin change plan: %v", err)
	}
	if previous != subscription.PlanFree {
		t.Errorf("expected previous plan code %q, got %q", subscription.PlanFree, previous)
	}
	if repo.changedTo != subscription.PlanPro {
		t.Errorf("expected repository to be asked to change to %q, got %q", subscription.PlanPro, repo.changedTo)
	}
}

// TestUnit_AdminChangePlan_UnknownPlan_RejectedBeforeWriting is the AC
// "plan_code tak dikenal ditolak" — and proves it is rejected BEFORE
// touching the repository at all, not written then discovered invalid.
func TestUnit_AdminChangePlan_UnknownPlan_RejectedBeforeWriting(t *testing.T) {
	repo := &fakeRepo{sub: &subscription.Subscription{PlanCode: subscription.PlanFree, Status: "active"}}
	u := subscription.NewUsecase(repo)

	_, err := u.AdminChangePlan(context.Background(), testTenant(), "not-a-real-plan")

	var verr *httpx.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected a validation error, got: %v", err)
	}
	if repo.changedTo != "" {
		t.Errorf("expected the repository to never be called for an unknown plan, but ChangePlan was asked for %q", repo.changedTo)
	}
}

func TestUnit_AdminChangePlan_NoActiveSubscription_PreviousIsEmpty(t *testing.T) {
	repo := &fakeRepo{err: subscription.ErrNoActiveSubscription}
	u := subscription.NewUsecase(repo)

	previous, err := u.AdminChangePlan(context.Background(), testTenant(), subscription.PlanPro)
	if err != nil {
		t.Fatalf("admin change plan: %v", err)
	}
	if previous != "" {
		t.Errorf("expected empty previous plan code, got %q", previous)
	}
}
