import { describe, expect, it } from "vitest";
import { LEAD_STATUSES, type LeadStatus } from "./labels";
import { isValidStatusTransition, statusTransitionOptions } from "./lead-status";

// This is the literal transition matrix from
// crm_be/internal/lead/usecase.go's validateStatusTransition, worked out
// by hand from the Go source (docs/phases/02-crm-core/td.md §5 + the
// documented "leaving lost" deviation) — not derived from
// isValidStatusTransition itself, so a bug that breaks BOTH the same way
// can't hide.
const EXPECTED_VALID: Record<LeadStatus, LeadStatus[]> = {
  new: ["contacted", "lost", "unqualified", "spam"],
  contacted: ["new", "qualified", "lost", "unqualified", "spam"],
  qualified: ["contacted", "proposal", "lost", "unqualified", "spam"],
  proposal: ["qualified", "won", "lost", "unqualified", "spam"],
  won: ["proposal", "lost", "unqualified", "spam"],
  // "leaving lost" is a documented backend simplification: ANY main-path
  // status is valid, not just the one the lead was in before — there's
  // no cheap way to know "before" without activity history (crm_be #20
  // notes). unqualified/spam are also reachable directly from lost.
  lost: ["new", "contacted", "qualified", "proposal", "won", "unqualified", "spam"],
  unqualified: [],
  spam: [],
};

describe("isValidStatusTransition", () => {
  it("matches the full matrix for every (from, to) pair — including same-status", () => {
    for (const from of LEAD_STATUSES) {
      for (const to of LEAD_STATUSES) {
        const expected = to === from ? to === "lost" : EXPECTED_VALID[from].includes(to);
        expect(
          isValidStatusTransition(from, to),
          `${from} -> ${to} expected ${expected}`
        ).toBe(expected);
      }
    }
  });

  it("unqualified and spam are final — no outgoing transition at all, not even to themselves", () => {
    for (const to of LEAD_STATUSES) {
      expect(isValidStatusTransition("unqualified", to)).toBe(false);
      expect(isValidStatusTransition("spam", to)).toBe(false);
    }
  });

  it("main path movement is exactly one step in either direction", () => {
    expect(isValidStatusTransition("qualified", "contacted")).toBe(true); // back
    expect(isValidStatusTransition("qualified", "proposal")).toBe(true); // forward
    expect(isValidStatusTransition("proposal", "new")).toBe(false); // two steps back
    expect(isValidStatusTransition("new", "won")).toBe(false); // skips ahead
  });
});

describe("statusTransitionOptions", () => {
  it("offers nothing for the two final statuses", () => {
    expect(statusTransitionOptions("unqualified")).toEqual([]);
    expect(statusTransitionOptions("spam")).toEqual([]);
  });

  it("offers only both main-path neighbors plus the three side exits for a middle status", () => {
    const options = statusTransitionOptions("qualified");
    expect(options.map((o) => o.status).sort()).toEqual(
      ["contacted", "proposal", "lost", "unqualified", "spam"].sort()
    );
  });

  it("offers only forward, no backward, for the first main-path status", () => {
    const options = statusTransitionOptions("new");
    const steps = options.filter((o) => o.kind === "step");
    expect(steps.map((o) => o.status)).toEqual(["contacted"]);
  });

  it("offers only backward, no forward, for the last main-path status", () => {
    const options = statusTransitionOptions("won");
    const steps = options.filter((o) => o.kind === "step");
    expect(steps.map((o) => o.status)).toEqual(["proposal"]);
  });

  it('restricts "lost" to a single reopen-to-"new" option, not all 5 valid main-path targets', () => {
    // isValidStatusTransition allows lost -> any main-path status; the UI
    // deliberately narrows this to avoid a wall of buttons for a rare
    // case. This test locks that narrowing as intentional.
    const options = statusTransitionOptions("lost");
    expect(options).toHaveLength(1);
    expect(options[0].status).toBe("new");
  });

  it("every option returned is actually valid per isValidStatusTransition", () => {
    for (const from of LEAD_STATUSES) {
      for (const option of statusTransitionOptions(from)) {
        expect(isValidStatusTransition(from, option.status)).toBe(true);
      }
    }
  });
});
