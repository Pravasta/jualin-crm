package subscription

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Usecase depends only on Repository (port.go), never on *pgxpool.Pool
// or pgx directly (ADR-011). It takes Repository directly rather than a
// Store — there is no transaction to open (same shape as
// internal/metrics, #30).
type Usecase struct {
	repo Repository
}

func NewUsecase(repo Repository) *Usecase {
	return &Usecase{repo: repo}
}

// ResolvePlan answers "what plan is t's organization on, and which
// channels does it open" — the single computation every other entry
// point in this package builds on. It never returns a non-nil error for
// "no active subscription": that state closes every channel (fail
// closed) rather than failing the caller (TD §1.1), since a caller like
// GET /v1/me must not go down because of one billing row.
func (u *Usecase) ResolvePlan(ctx context.Context, t tenant.Context) (code string, channels map[string]bool, err error) {
	sub, err := u.repo.FindActiveByOrg(ctx, t)
	if err != nil {
		if errors.Is(err, ErrNoActiveSubscription) {
			return "", stringKeyed(channelsFor("", "")), nil
		}
		return "", nil, fmt.Errorf("subscription: resolve plan: %w", err)
	}

	return sub.PlanCode, stringKeyed(channelsFor(sub.PlanCode, sub.Status)), nil
}

// RequireChannel returns nil when t's organization's plan opens ch, and
// a 403 plan_upgrade_required *httpx.DomainError otherwise. It has zero
// callers as of this issue (#112) — apikey/form/webhook wire it up in
// #113, mirroring the precedent of apikey.FindByKeyID (#46),
// form.FindByPublicKey (#85), and webhook.ClaimDue/Reap/Purge (#100):
// built one issue before whatever calls it.
func (u *Usecase) RequireChannel(ctx context.Context, t tenant.Context, ch Channel) error {
	sub, err := u.repo.FindActiveByOrg(ctx, t)
	if err != nil && !errors.Is(err, ErrNoActiveSubscription) {
		return fmt.Errorf("subscription: require channel: %w", err)
	}

	var planCode, status string
	if sub != nil {
		planCode, status = sub.PlanCode, sub.Status
	}

	if !channelsFor(planCode, status)[ch] {
		return planUpgradeRequiredError()
	}
	return nil
}

// stringKeyed converts the internal Channel-keyed map to the
// string-keyed shape every consumer-declared PlanGate interface (§3.2)
// and GET /v1/me's JSON body (§4) expect. Channel stays unexported to
// callers outside this package on purpose: TD §3.2 requires
// apikey/form/webhook to call RequireChannel with a plain string
// literal rather than importing subscription.Channel, precisely so they
// never need to import this package at all.
func stringKeyed(in map[Channel]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for ch, ok := range in {
		out[string(ch)] = ok
	}
	return out
}

func planUpgradeRequiredError() error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "plan_upgrade_required",
		Message: "Paket Anda tidak mencakup kanal ini.",
	}
}
