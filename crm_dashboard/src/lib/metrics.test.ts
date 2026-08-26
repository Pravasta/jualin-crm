import { describe, expect, it } from "vitest";
import { statusCount, type MetricsSummary } from "./metrics";

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
