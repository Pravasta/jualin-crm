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

// ResolvePlan answers "what plan is t's organization on, which channels
// does it open, and how much does it allow" — the single computation
// every other entry point in this package builds on. It never returns a
// non-nil error for "no active subscription": that state closes every
// channel and drops to free-tier quantities (TD §1.1, 8.5 §2.1) rather
// than failing the caller, since GET /v1/me must not go down because of
// one billing row.
//
// Limits is returned as the domain type rather than flattened into
// primitives the way channels is. internal/auth already imports this
// package for *subscription.Subscription, so there is nothing to avoid
// here — and two ints returned positionally would be exactly the kind
// of call site where leads and seats get swapped without the compiler
// noticing.
func (u *Usecase) ResolvePlan(ctx context.Context, t tenant.Context) (code string, channels map[string]bool, limits Limits, err error) {
	sub, err := u.repo.FindActiveByOrg(ctx, t)
	if err != nil {
		if errors.Is(err, ErrNoActiveSubscription) {
			return "", stringKeyed(channelsFor("", "")), limitsFor("", ""), nil
		}
		return "", nil, Limits{}, fmt.Errorf("subscription: resolve plan: %w", err)
	}

	return sub.PlanCode,
		stringKeyed(channelsFor(sub.PlanCode, sub.Status)),
		limitsFor(sub.PlanCode, sub.Status),
		nil
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

// RequireLeadQuota returns nil when used more lead fits under t's
// organization's monthly quota (Phase 8.5 #123), and a 403
// plan_quota_exceeded *httpx.DomainError otherwise.
//
// Unlike RequireChannel's deliberately vague message, this one names
// the actual number. The two calls answer different questions to
// different audiences: RequireChannel can be reached by a Manager who
// has no business learning the organization's billing state at all
// (subscription TD §5, "jangan menjawab pertanyaan yang penanyanya
// tidak berhak ajukan"); RequireLeadQuota is reached by the
// organization's OWN authenticated principal (user or api_key — never
// public_form, TD 8.5 §5) asking about ITS OWN usage. Vagueness there
// protects the org from an outsider; vagueness here would just leave
// its own customer guessing.
func (u *Usecase) RequireLeadQuota(ctx context.Context, t tenant.Context, used int) error {
	sub, err := u.repo.FindActiveByOrg(ctx, t)
	if err != nil && !errors.Is(err, ErrNoActiveSubscription) {
		return fmt.Errorf("subscription: require lead quota: %w", err)
	}

	var planCode, status string
	if sub != nil {
		planCode, status = sub.PlanCode, sub.Status
	}

	limits := limitsFor(planCode, status)
	if !allows(limits.LeadsPerMonth, used) {
		return planQuotaExceededError(limits.LeadsPerMonth)
	}
	return nil
}

// RequireSeatLimit returns nil when used more member fits under t's
// organization's seat limit (Phase 8.5 #124), and a 403
// plan_seat_limit_reached *httpx.DomainError otherwise. Same shape as
// RequireLeadQuota — the audience is the organization's own Owner/Admin
// inviting someone, so the message names the actual number.
func (u *Usecase) RequireSeatLimit(ctx context.Context, t tenant.Context, used int) error {
	sub, err := u.repo.FindActiveByOrg(ctx, t)
	if err != nil && !errors.Is(err, ErrNoActiveSubscription) {
		return fmt.Errorf("subscription: require seat limit: %w", err)
	}

	var planCode, status string
	if sub != nil {
		planCode, status = sub.PlanCode, sub.Status
	}

	limits := limitsFor(planCode, status)
	if !allows(limits.Seats, used) {
		return planSeatLimitReachedError(limits.Seats)
	}
	return nil
}

// AdminChangePlan sets t.OrganizationID's plan_code (Phase 8.5 #124) —
// called only by the two admin surfaces in cmd/api (see their own doc
// comments for why: this is deliberately NOT a normal user-facing
// write). Returns the plan code that was active before the change, so
// the caller can write an audit_log entry with both old_values and
// new_values without a second read.
//
// planCode is validated against planChannels — not against planLimits,
// though the two are kept in lockstep by TestPlanLimits_EveryPlanHasEntry
// — because "is this a plan this codebase knows about at all" is one
// question asked once, not two. A typo here must be rejected outright,
// never silently accepted and left to resolve as "unknown plan" behavior
// (§2.1) the next time anything reads it back.
func (u *Usecase) AdminChangePlan(ctx context.Context, t tenant.Context, planCode string) (previousPlanCode string, err error) {
	if _, ok := planChannels[planCode]; !ok {
		return "", unknownPlanCodeError()
	}

	sub, err := u.repo.FindActiveByOrg(ctx, t)
	if err != nil && !errors.Is(err, ErrNoActiveSubscription) {
		return "", fmt.Errorf("subscription: admin change plan: find current: %w", err)
	}
	if sub != nil {
		previousPlanCode = sub.PlanCode
	}

	if err := u.repo.ChangePlan(ctx, t, planCode); err != nil {
		return "", fmt.Errorf("subscription: admin change plan: %w", err)
	}
	return previousPlanCode, nil
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

func planQuotaExceededError(limit int) error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "plan_quota_exceeded",
		Message: fmt.Sprintf("Paket Anda dibatasi %d lead per bulan. Sudah tercapai untuk bulan ini.", limit),
	}
}

func planSeatLimitReachedError(limit int) error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "plan_seat_limit_reached",
		Message: fmt.Sprintf("Paket Anda dibatasi %d anggota. Sudah tercapai batasnya.", limit),
	}
}

func unknownPlanCodeError() error {
	return httpx.NewValidationError(httpx.ErrorDetail{Field: "plan_code", Code: "invalid_value"})
}
