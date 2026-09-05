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
	sub *subscription.Subscription
	err error
}

func (f *fakeRepo) FindActiveByOrg(_ context.Context, _ tenant.Context) (*subscription.Subscription, error) {
	return f.sub, f.err
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
