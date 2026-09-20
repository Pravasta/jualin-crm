import { describe, expect, it } from "vitest";
import { LEAD_STATUSES, type LeadStatus } from "./labels";
import { hasBeenConverted, isValidStatusTransition, statusTransitionOptions } from "./lead-status";

// This is the literal transition matrix from
// crm_be/internal/lead/usecase.go's validateStatusTransition, worked out
// by hand from the Go source (docs/phases/02-crm-core/td.md §5 + the
// documented "leaving lost" deviation) — not derived from
// isValidStatusTransition itself, so a bug that breaks BOTH the same way
// can't hide.
//
// Nothing leads to "new" (issue #139, ADR-015): "new" means "belum
// disentuh", a statement about history rather than a stage of work, so no
// row below lists it — not even lost's, which is why reopening goes to
// "contacted".
const EXPECTED_VALID: Record<LeadStatus, LeadStatus[]> = {
  new: ["contacted", "lost", "unqualified", "spam"],
  contacted: ["qualified", "lost", "unqualified", "spam"],
  qualified: ["contacted", "proposal", "lost", "unqualified", "spam"],
  proposal: ["qualified", "won", "lost", "unqualified", "spam"],
  won: ["proposal", "lost", "unqualified", "spam"],
  // "leaving lost" is a documented backend simplification: ANY main-path
  // status is valid, not just the one the lead was in before — there's
  // no cheap way to know "before" without activity history (crm_be #20
  // notes). unqualified/spam are also reachable directly from lost.
  lost: ["contacted", "qualified", "proposal", "won", "unqualified", "spam"],
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
    expect(isValidStatusTransition("proposal", "qualified")).toBe(true); // back
    expect(isValidStatusTransition("won", "proposal")).toBe(true); // back
    expect(isValidStatusTransition("proposal", "new")).toBe(false); // two steps back
    expect(isValidStatusTransition("new", "won")).toBe(false); // skips ahead
  });
});

describe("nothing leads back to new (issue #139)", () => {
  it("no status can move to new — checked per status, not only via the matrix", () => {
    for (const from of LEAD_STATUSES) {
      expect(isValidStatusTransition(from, "new"), `${from} -> new`).toBe(false);
    }
  });

  it("the UI never offers a way into new, from any status", () => {
    for (const from of LEAD_STATUSES) {
      const targets = statusTransitionOptions(from).map((o) => o.status);
      expect(targets, `options from ${from}`).not.toContain("new");
    }
  });

  it('"contacted" offers forward and the side exits, but no backward step', () => {
    const steps = statusTransitionOptions("contacted").filter((o) => o.direction === "forward" || o.direction === "back");
    expect(steps.map((o) => o.status)).toEqual(["qualified"]);
    expect(steps.map((o) => o.direction)).toEqual(["forward"]);
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
    const steps = options.filter((o) => o.direction === "forward" || o.direction === "back");
    expect(steps.map((o) => o.status)).toEqual(["contacted"]);
    expect(steps.map((o) => o.direction)).toEqual(["forward"]);
  });

  it("offers only backward, no forward, for the last main-path status", () => {
    const options = statusTransitionOptions("won");
    const steps = options.filter((o) => o.direction === "forward" || o.direction === "back");
    expect(steps.map((o) => o.status)).toEqual(["proposal"]);
    expect(steps.map((o) => o.direction)).toEqual(["back"]);
  });

  it('restricts "lost" to a single reopen-to-"contacted" option, not all 5 valid main-path targets', () => {
    // isValidStatusTransition allows lost -> any main-path status except
    // new; the UI deliberately narrows this to avoid a wall of buttons for
    // a rare case. This test locks that narrowing as intentional — and the
    // target: "new" is closed (ADR-015), "contacted" is the earliest stage
    // a reopened lead can honestly be in.
    const options = statusTransitionOptions("lost");
    expect(options).toHaveLength(1);
    expect(options[0].status).toBe("contacted");
    expect(options[0].direction).toBe("reopen");
    expect(options[0].label).toBe("Buka kembali ke Dihubungi");
  });

  it("every option returned is actually valid per isValidStatusTransition", () => {
    for (const from of LEAD_STATUSES) {
      for (const option of statusTransitionOptions(from)) {
        expect(isValidStatusTransition(from, option.status)).toBe(true);
      }
    }
  });
});

// Issue #142. The timeline's "lead_converted" entry is the only "already
// converted" signal the client has (the Lead carries no such flag), so the
// predicate that drives "stop offering the status buttons" is worth pinning.
describe("hasBeenConverted", () => {
  it("is true once the timeline carries a lead_converted entry, wherever it sits", () => {
    expect(hasBeenConverted([{ type: "lead_created" }, { type: "lead_converted" }])).toBe(true);
    expect(hasBeenConverted([{ type: "lead_converted" }, { type: "status_changed" }])).toBe(true);
  });

  it("is false for a timeline without one — including a won lead that was never converted", () => {
    expect(hasBeenConverted([])).toBe(false);
    expect(
      hasBeenConverted([{ type: "lead_created" }, { type: "status_changed" }, { type: "note" }])
    ).toBe(false);
  });
});

// Issue #141. The old labels put "→" on EVERY step, forward or back, so
// "→ Memenuhi Syarat" at Penawaran pointed forward while going backward. The
// direction now travels as data and the label is a verb — no arrow at all, so
// there is nothing left to read the wrong way.
describe("direction and labels (issue #141)", () => {
  it("every option carries a direction that matches where it actually goes", () => {
    const order: LeadStatus[] = ["new", "contacted", "qualified", "proposal", "won"];
    for (const from of LEAD_STATUSES) {
      for (const opt of statusTransitionOptions(from)) {
        const f = order.indexOf(from);
        const t = order.indexOf(opt.status);
        if (opt.direction === "forward") expect(t, `${from} -> ${opt.status}`).toBeGreaterThan(f);
        if (opt.direction === "back") expect(t, `${from} -> ${opt.status}`).toBeLessThan(f);
        if (opt.direction === "close") expect(["lost", "unqualified", "spam"]).toContain(opt.status);
        if (opt.direction === "reopen") expect(from).toBe("lost");
      }
    }
  });

  it("labels are verbs, and no label uses an arrow for either direction", () => {
    for (const from of LEAD_STATUSES) {
      for (const opt of statusTransitionOptions(from)) {
        expect(opt.label, `${from} -> ${opt.status}`).not.toMatch(/[→←↑↓]/);
        if (opt.direction === "forward") expect(opt.label).toMatch(/^Maju ke /);
        if (opt.direction === "back") expect(opt.label).toMatch(/^Kembali ke /);
        if (opt.direction === "reopen") expect(opt.label).toMatch(/^Buka kembali ke /);
      }
    }
  });

  it("names the destination the way the badge does — one word for one status", () => {
    const won = statusTransitionOptions("proposal").find((o) => o.status === "won");
    expect(won?.label).toBe("Maju ke Menang");
    const back = statusTransitionOptions("proposal").find((o) => o.status === "qualified");
    expect(back?.label).toBe("Kembali ke Memenuhi Syarat");
  });
});
