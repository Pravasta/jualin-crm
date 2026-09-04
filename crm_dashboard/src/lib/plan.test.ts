import { describe, expect, it } from "vitest";
import { ApiError } from "./api-types";
import type { MeResponse } from "./auth";
import { PLAN_CHANNELS, channelCardState, isChannelOpen, isPlanUpgradeRequired } from "./plan";

function planWith(channels: Record<string, boolean>): MeResponse["plan"] {
  return { code: "free", channels };
}

describe("PLAN_CHANNELS", () => {
  // Wire contract with crm_be's subscription.Channels (subscription
  // TD §7) — a rename on either side must fail here, not silently leave
  // one channel unreadable.
  it("matches the backend's Channels exactly", () => {
    expect([...PLAN_CHANNELS]).toEqual(["api_key", "form", "webhook"]);
  });
});

describe("isChannelOpen", () => {
  it("is true only when the key is exactly true", () => {
    expect(isChannelOpen(planWith({ webhook: true }), "webhook")).toBe(true);
    expect(isChannelOpen(planWith({ webhook: false }), "webhook")).toBe(false);
  });

  // The fail-closed guarantee: a key the backend never sent (typo,
  // channel added on one side and not the other, plan resolution
  // failure) must read as closed, never as open.
  it("treats a missing key as closed", () => {
    expect(isChannelOpen(planWith({}), "webhook")).toBe(false);
  });

  // Every member of PLAN_CHANNELS, table-driven — a channel added later
  // is covered automatically rather than needing a new hand-written case.
  it("covers every channel in the table", () => {
    for (const channel of PLAN_CHANNELS) {
      expect(isChannelOpen(planWith({ [channel]: true }), channel)).toBe(true);
      expect(isChannelOpen(planWith({}), channel)).toBe(false);
    }
  });
});

describe("channelCardState", () => {
  it("is unavailable regardless of plan when the product hasn't shipped the channel", () => {
    const state = channelCardState("unavailable", planWith({ webhook: true }), "webhook");
    expect(state).toBe("unavailable");
  });

  it("is active when the product ships it and the plan opens it", () => {
    expect(channelCardState("active", planWith({ webhook: true }), "webhook")).toBe("active");
  });

  it("is locked, not unavailable, when the product ships it but the plan closes it", () => {
    expect(channelCardState("active", planWith({ webhook: false }), "webhook")).toBe("locked");
  });

  it("is locked when the plan sends no key for the channel at all", () => {
    expect(channelCardState("active", planWith({}), "webhook")).toBe("locked");
  });
});

describe("isPlanUpgradeRequired", () => {
  it("recognizes plan_upgrade_required", () => {
    const err = new ApiError(403, { code: "plan_upgrade_required", message: "Paket Anda tidak mencakup kanal ini." });
    expect(isPlanUpgradeRequired(err)).toBe(true);
  });

  it("rejects other ApiErrors and non-ApiErrors", () => {
    expect(isPlanUpgradeRequired(new ApiError(403, { code: "forbidden", message: "x" }))).toBe(false);
    expect(isPlanUpgradeRequired(new Error("network error"))).toBe(false);
    expect(isPlanUpgradeRequired(null)).toBe(false);
  });
});
