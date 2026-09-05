import { describe, expect, it } from "vitest";
import { ApiError } from "./api-types";
import type { MeResponse } from "./auth";
import {
  PLAN_CHANNELS,
  channelCardState,
  formatLimit,
  formatUsage,
  isChannelOpen,
  isPlanQuotaExceeded,
  isPlanSeatLimitReached,
  isPlanUpgradeRequired,
  isUnlimitedLimit,
  planDisplayName,
  usageRatio,
} from "./plan";

function planWith(channels: Record<string, boolean>): MeResponse["plan"] {
  return {
    code: "free",
    channels,
    limits: { leads_per_month: 100, seats: 2 },
    usage: { leads_this_month: 0, seats_used: 0 },
    test_checkout_available: false,
  };
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

describe("isPlanQuotaExceeded", () => {
  it("recognizes plan_quota_exceeded and nothing else", () => {
    const err = new ApiError(403, { code: "plan_quota_exceeded", message: "x" });
    expect(isPlanQuotaExceeded(err)).toBe(true);
    expect(isPlanQuotaExceeded(new ApiError(403, { code: "plan_seat_limit_reached", message: "x" }))).toBe(false);
    expect(isPlanQuotaExceeded(null)).toBe(false);
  });
});

describe("isPlanSeatLimitReached", () => {
  it("recognizes plan_seat_limit_reached and nothing else", () => {
    const err = new ApiError(403, { code: "plan_seat_limit_reached", message: "x" });
    expect(isPlanSeatLimitReached(err)).toBe(true);
    expect(isPlanSeatLimitReached(new ApiError(403, { code: "plan_quota_exceeded", message: "x" }))).toBe(false);
    expect(isPlanSeatLimitReached(null)).toBe(false);
  });
});

describe("planDisplayName", () => {
  it("names all three known plans", () => {
    expect(planDisplayName("free")).toBe("Free");
    expect(planDisplayName("pro")).toBe("Pro");
    expect(planDisplayName("enterprise")).toBe("Enterprise");
  });

  // An organization's plan_code this build doesn't recognize must still
  // render something, not crash the screen.
  it("falls back to the raw code for an unrecognized plan", () => {
    expect(planDisplayName("mystery-plan")).toBe("mystery-plan");
  });
});

describe("isUnlimitedLimit / formatLimit / formatUsage", () => {
  it("0 means unlimited, never zero quota", () => {
    expect(isUnlimitedLimit(0)).toBe(true);
    expect(isUnlimitedLimit(100)).toBe(false);
    expect(formatLimit(0)).toBe("Tanpa batas");
    expect(formatLimit(100)).toBe("100");
  });

  it("formatUsage shows the fraction for a limited plan, and just the count for unlimited", () => {
    expect(formatUsage(37, 100)).toBe("37 / 100");
    expect(formatUsage(37, 0)).toBe("37 (tanpa batas)");
  });
});

describe("usageRatio", () => {
  it("computes a normal in-range fraction", () => {
    expect(usageRatio(50, 100)).toBe(0.5);
    expect(usageRatio(0, 100)).toBe(0);
  });

  // The AC verbatim: usage past the limit must not render as more than
  // full, and must never go negative.
  it("clamps usage past the limit to 1, never more", () => {
    expect(usageRatio(150, 100)).toBe(1);
  });

  it("is 0 for an unlimited plan rather than a division by zero", () => {
    expect(usageRatio(37, 0)).toBe(0);
  });

  it("never goes negative even for a nonsensical negative input", () => {
    expect(usageRatio(-5, 100)).toBe(0);
  });
});
