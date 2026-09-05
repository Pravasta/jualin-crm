package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/subscription"
)

// planGate is the composition root's bridge from subscription.Usecase to
// apikey.PlanGate, form.PlanGate, and webhook.PlanGate — one value
// satisfies all three structurally, the same bridging pattern as
// activity.NewRecorder / webhook.NewEnqueuer. None of the three domain
// packages imports internal/subscription (ADR-011); only this file does.
type planGate struct {
	usecase *subscription.Usecase
	logger  *slog.Logger
}

func newPlanGate(usecase *subscription.Usecase, logger *slog.Logger) *planGate {
	return &planGate{usecase: usecase, logger: logger}
}

// RequireChannel parses ch through subscription.ParseChannel rather than
// converting blindly with subscription.Channel(ch) (TD §7's own warning:
// a typo in one of the four places this literal is duplicated makes that
// channel silently ALWAYS closed — indistinguishable from an honest
// deny). An unrecognized ch is a wiring bug in this codebase, not a
// customer's plan state, so it is logged loudly and returned as an
// opaque error rather than mapped to 403 plan_upgrade_required.
func (g *planGate) RequireChannel(ctx context.Context, t tenant.Context, ch string) error {
	parsed, err := subscription.ParseChannel(ch)
	if err != nil {
		g.logger.Error("subscription: plan gate called with unknown channel literal", "channel", ch, "err", err)
		return fmt.Errorf("subscription: plan gate: %w", err)
	}
	return g.usecase.RequireChannel(ctx, t, parsed)
}

// AllowLead bridges *subscription.Usecase.RequireLeadQuota (#123) to
// lead.PlanQuota — the same planGate value wired for the three channel
// gates above satisfies this interface too, structurally.
func (g *planGate) AllowLead(ctx context.Context, t tenant.Context, used int) error {
	return g.usecase.RequireLeadQuota(ctx, t, used)
}

// AllowSeat bridges *subscription.Usecase.RequireSeatLimit (#124) to
// invitation.PlanSeatQuota — the same planGate value satisfies this
// interface too, structurally.
func (g *planGate) AllowSeat(ctx context.Context, t tenant.Context, used int) error {
	return g.usecase.RequireSeatLimit(ctx, t, used)
}
