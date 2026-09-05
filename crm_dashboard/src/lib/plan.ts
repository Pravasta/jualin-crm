// Reads the capability answer GET /v1/me already sends (subscription
// TD §4, D5) — this file NEVER decides which plan opens which channel.
// That decision is crm_be's alone (internal/subscription/plan.go's
// planChannels); duplicating it here as a second map is exactly the
// mistake #33 had to correct in lib/lead-status.ts.
import { ApiError } from "./api-types";
import type { MeResponse } from "./auth";

// PLAN_CHANNELS mirrors subscription.Channels (crm_be) — the wire
// contract subscription TD §7 requires locking with a test, same shape
// as WEBHOOK_EVENTS vs webhook.KnownEvents (#103). Adding a fourth
// channel means adding one entry here; every consumer keyed off this
// array picks it up automatically.
export const PLAN_CHANNELS = ["api_key", "form", "webhook"] as const;

export type PlanChannel = (typeof PLAN_CHANNELS)[number];

// isChannelOpen fails CLOSED: a channel absent from plan.channels (a
// key the backend never sent, or a typo somewhere in the four places
// this literal is duplicated — subscription TD §7) is treated as
// closed, never open. `=== true` rather than a truthy check makes that
// explicit rather than accidental.
export function isChannelOpen(plan: MeResponse["plan"], channel: PlanChannel): boolean {
  return plan.channels[channel] === true;
}

export type ChannelCardState = "active" | "locked" | "unavailable";

// channelCardState combines two INDEPENDENT facts, and conflating them
// is the bug this function exists to prevent: whether the product has
// built this channel at all (productStatus — a fact this screen already
// knows per card, unrelated to any organization's plan) and whether
// this organization's plan currently opens it. A channel the product
// hasn't shipped yet must never read as "locked" — that would claim a
// billing reason for something that simply doesn't exist (06/td.md §416).
export function channelCardState(
  productStatus: "active" | "unavailable",
  plan: MeResponse["plan"],
  channel: PlanChannel
): ChannelCardState {
  if (productStatus === "unavailable") return "unavailable";
  return isChannelOpen(plan, channel) ? "active" : "locked";
}

// isPlanUpgradeRequired is the 403 a create call can get from a RACE
// (the plan closed between render and click) — same shape as
// isWebhookUrlNotAllowed (#103). Callers steer it into the same banner
// globalMessage(err) already renders; this helper exists so that
// handling is a visible, tested decision rather than something that
// happens to work because the error falls into a catch-all.
export function isPlanUpgradeRequired(err: unknown): boolean {
  return err instanceof ApiError && err.code === "plan_upgrade_required";
}

// isPlanQuotaExceeded (subscription #123) and isPlanSeatLimitReached
// (#124) are the same shape as isPlanUpgradeRequired above, for the two
// codes #125 asks to be shown INLINE at their point of origin (new-lead
// form, invite dialog) rather than as a generic banner with no next
// step.
export function isPlanQuotaExceeded(err: unknown): boolean {
  return err instanceof ApiError && err.code === "plan_quota_exceeded";
}

export function isPlanSeatLimitReached(err: unknown): boolean {
  return err instanceof ApiError && err.code === "plan_seat_limit_reached";
}

// PLAN_NAMES / planDisplayName is #125's — the FIRST reader of
// plan.code anywhere in this product, closing docs/issues/112's open
// point. Falls back to the raw code for anything unrecognized rather
// than throwing: an organization sitting on a plan_code this build
// doesn't know about (a value the payment service writes later, or a
// retired plan) must still render a screen, not crash it.
const PLAN_NAMES: Record<string, string> = {
  free: "Free",
  pro: "Pro",
  enterprise: "Enterprise",
};

export function planDisplayName(code: string): string {
  return PLAN_NAMES[code] ?? code;
}

// isUnlimitedLimit / formatLimit / formatUsage read the ONE meaning of
// 0 inside limits.leads_per_month / limits.seats (subscription TD §7:
// "limits: 0 berarti tanpa batas") — every place on this screen that
// needs to know renders through these, never `=== 0` inline, mirroring
// crm_be's allows() being the one place Go reads it.
export function isUnlimitedLimit(limit: number): boolean {
  return limit === 0;
}

export function formatLimit(limit: number): string {
  return isUnlimitedLimit(limit) ? "Tanpa batas" : String(limit);
}

export function formatUsage(used: number, limit: number): string {
  return isUnlimitedLimit(limit) ? `${used} (tanpa batas)` : `${used} / ${limit}`;
}

// usageRatio is the ONLY thing a progress bar reads — clamped to [0, 1]
// so usage past its limit (a public form lead created after quota
// closed, #123's D3; a seat invited right before a manual downgrade)
// never renders as more-than-full or, if either number were ever
// negative, as a backwards bar (#125 AC: "tidak menghasilkan angka
// negatif atau bar >100% yang aneh"). Unlimited has no "full" to show —
// callers check isUnlimitedLimit first and skip the bar entirely rather
// than asking this for a meaningless ratio.
export function usageRatio(used: number, limit: number): number {
  if (isUnlimitedLimit(limit) || limit < 0) return 0;
  return Math.min(1, Math.max(0, used / limit));
}
