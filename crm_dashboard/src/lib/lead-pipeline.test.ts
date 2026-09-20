import { describe, expect, it } from "vitest";
import { LEAD_STATUSES, type LeadStatus } from "./labels";
import { actionGroups, closedNote, isFinalStatus, pipelineSteps, type PipelineStepState } from "./lead-pipeline";

// Issue #141. Everything below is worked out by hand from the design in the
// issue, not derived from the functions under test, so a bug that breaks the
// implementation and its expectation the same way cannot hide. The component
// that renders this is deliberately thin — Vitest only loads *.test.ts, so
// what can be silently wrong has to live here.

const MAIN_PATH: LeadStatus[] = ["new", "contacted", "qualified", "proposal", "won"];

const EXPECTED_STEPS: Record<string, PipelineStepState[]> = {
  new: ["current", "upcoming", "upcoming", "upcoming", "upcoming"],
  contacted: ["done", "current", "upcoming", "upcoming", "upcoming"],
  qualified: ["done", "done", "current", "upcoming", "upcoming"],
  proposal: ["done", "done", "done", "current", "upcoming"],
  won: ["done", "done", "done", "done", "current"],
};

describe("pipelineSteps", () => {
  it("always lists the five main-path stages in order, with the badge's own labels", () => {
    for (const status of LEAD_STATUSES) {
      const steps = pipelineSteps(status);
      expect(steps.map((s) => s.status)).toEqual(MAIN_PATH);
    }
    expect(pipelineSteps("qualified").map((s) => s.label)).toEqual([
      "Baru",
      "Dihubungi",
      "Memenuhi Syarat",
      "Penawaran",
      "Menang",
    ]);
  });

  it("marks done / current / upcoming for each main-path status", () => {
    for (const [status, expected] of Object.entries(EXPECTED_STEPS)) {
      expect(pipelineSteps(status as LeadStatus).map((s) => s.state), status).toEqual(expected);
    }
  });

  it("has exactly one current step on the main path — never zero, never two", () => {
    for (const status of MAIN_PATH) {
      expect(pipelineSteps(status).filter((s) => s.state === "current")).toHaveLength(1);
    }
  });

  it("does not invent a position for a closed lead: every step is inactive", () => {
    for (const status of ["lost", "unqualified", "spam"] as const) {
      const states = pipelineSteps(status).map((s) => s.state);
      expect(states, status).toEqual(["inactive", "inactive", "inactive", "inactive", "inactive"]);
    }
  });
});

describe("actionGroups", () => {
  const titles = (s: LeadStatus) => actionGroups(s).map((g) => g.title);
  const targets = (s: LeadStatus, key: string) =>
    actionGroups(s)
      .find((g) => g.key === key)
      ?.actions.map((a) => a.status);

  it("new: forward to Dihubungi and the three closers — there is no way back", () => {
    expect(titles("new")).toEqual(["Lanjutkan", "Tutup lead"]);
    expect(targets("new", "forward")).toEqual(["contacted"]);
    expect(targets("new", "close")).toEqual(["lost", "unqualified", "spam"]);
  });

  it("contacted: forward and closers, and NO Kembali section (ADR-015)", () => {
    expect(titles("contacted")).toEqual(["Lanjutkan", "Tutup lead"]);
    expect(targets("contacted", "forward")).toEqual(["qualified"]);
  });

  it("qualified and proposal: forward, back, and the closers", () => {
    expect(titles("qualified")).toEqual(["Lanjutkan", "Kembali", "Tutup lead"]);
    expect(targets("qualified", "forward")).toEqual(["proposal"]);
    expect(targets("qualified", "back")).toEqual(["contacted"]);
    expect(targets("proposal", "forward")).toEqual(["won"]);
    expect(targets("proposal", "back")).toEqual(["qualified"]);
  });

  it("won: no Lanjutkan section — conversion is its own control, not a status", () => {
    expect(titles("won")).toEqual(["Kembali", "Tutup lead"]);
    expect(targets("won", "back")).toEqual(["proposal"]);
  });

  it("lost: only Buka kembali, to Dihubungi", () => {
    expect(titles("lost")).toEqual(["Buka kembali"]);
    expect(targets("lost", "reopen")).toEqual(["contacted"]);
  });

  it("unqualified and spam are final: no sections at all", () => {
    expect(actionGroups("unqualified")).toEqual([]);
    expect(actionGroups("spam")).toEqual([]);
  });

  it("never returns an empty section — an empty one would render a heading over nothing", () => {
    for (const status of LEAD_STATUSES) {
      for (const group of actionGroups(status)) {
        expect(group.actions.length, `${status} / ${group.key}`).toBeGreaterThan(0);
      }
    }
  });

  it("sections come in a fixed order regardless of status", () => {
    const order = ["forward", "back", "reopen", "close"];
    for (const status of LEAD_STATUSES) {
      const keys = actionGroups(status).map((g) => g.key);
      expect(keys, status).toEqual(order.filter((k) => keys.includes(k as never)));
    }
  });
});

describe("closedNote and isFinalStatus", () => {
  it("says what closed the lead, in the badge's own words", () => {
    expect(closedNote("lost")).toBe("Lead ditutup: Kalah");
    expect(closedNote("unqualified")).toBe("Lead ditutup: Tidak Memenuhi Syarat");
    expect(closedNote("spam")).toBe("Lead ditutup: Spam");
  });

  it("is null for every status still on the main path", () => {
    for (const status of MAIN_PATH) expect(closedNote(status)).toBeNull();
  });

  it("only unqualified and spam are final — lost can be reopened", () => {
    for (const status of LEAD_STATUSES) {
      expect(isFinalStatus(status), status).toBe(status === "unqualified" || status === "spam");
    }
  });
});
