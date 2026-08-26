import { describe, expect, it } from "vitest";
import {
  formatAvgResponseSeconds,
  formatConversionRate,
  periodToRange,
  statusCount,
  type MetricsSummary,
} from "./metrics";

describe("statusCount", () => {
  it('shows the loading placeholder while summary itself has not loaded', () => {
    expect(statusCount(null, "new")).toBe("…");
  });

  // Found verifying against a real crm_be (notes.md "## #32"): by_status
  // is a Go map, and a status with zero leads is OMITTED from the JSON
  // entirely — GET /v1/metrics/summary returned
  // {"by_status":{"contacted":1,"new":4}} for an org with 5 leads across
  // only two statuses. A naive `summary.by_status[status] ?? "…"` would
  // render "…" forever for "won", "lost", and every status nobody has
  // used yet — indistinguishable from a stuck loading state.
  it("resolves a status ABSENT from a loaded summary to 0, not the loading placeholder", () => {
    const summary: MetricsSummary = {
      total_new: 5,
      by_status: { new: 4, contacted: 1 },
      unassigned: 4,
      conversion_rate: 0,
    };
    expect(statusCount(summary, "won")).toBe(0);
    expect(statusCount(summary, "spam")).toBe(0);
  });

  it("resolves a status present in a loaded summary to its real count", () => {
    const summary: MetricsSummary = {
      total_new: 5,
      by_status: { new: 4, contacted: 1 },
      unassigned: 4,
      conversion_rate: 0,
    };
    expect(statusCount(summary, "new")).toBe(4);
    expect(statusCount(summary, "contacted")).toBe(1);
  });
});

describe("formatConversionRate", () => {
  // conversion_rate is a raw fraction (won / (total - spam - unqualified)),
  // never a percentage already — internal/metrics/repository_postgres.go
  // divides two counts directly with no ×100.
  it("renders a fraction as a rounded percentage", () => {
    expect(formatConversionRate(0.42)).toBe("42%");
    expect(formatConversionRate(1)).toBe("100%");
    expect(formatConversionRate(0)).toBe("0%");
  });

  // null (denominator was zero) must read differently from a real 0% —
  // issue #35's AC and TD §2.2 are both explicit about this, and it's
  // the exact class of bug #32 found in by_status (missing key silently
  // read as "…" forever instead of a real 0).
  it("renders null as 'no data', distinct from a real 0%", () => {
    expect(formatConversionRate(null)).toBe("Belum ada data");
  });
});

describe("formatAvgResponseSeconds", () => {
  it("renders null as an em dash, not '0 menit'", () => {
    expect(formatAvgResponseSeconds(null)).toBe("—");
  });

  it("renders sub-hour durations in minutes", () => {
    expect(formatAvgResponseSeconds(90)).toBe("2 menit");
    expect(formatAvgResponseSeconds(3599)).toBe("60 menit");
  });

  it("renders hour-plus durations in hours", () => {
    expect(formatAvgResponseSeconds(3600)).toBe("1 jam");
    expect(formatAvgResponseSeconds(7200)).toBe("2 jam");
  });
});

describe("periodToRange", () => {
  it("computes a UTC range ending at 'now' and starting N days earlier", () => {
    const now = new Date("2026-08-27T12:00:00.000Z");
    expect(periodToRange("7d", now)).toEqual({
      from: "2026-08-20T12:00:00.000Z",
      to: "2026-08-27T12:00:00.000Z",
    });
    expect(periodToRange("30d", now)).toEqual({
      from: "2026-07-28T12:00:00.000Z",
      to: "2026-08-27T12:00:00.000Z",
    });
  });
});
