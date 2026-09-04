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
