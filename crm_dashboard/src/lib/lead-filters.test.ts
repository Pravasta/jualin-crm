import { describe, expect, it } from "vitest";
import { hasAnyLeadFilter, parseCSVParam, toggleCSVValue } from "./lead-filters";

describe("parseCSVParam", () => {
  it("splits a comma-separated URL param", () => {
    expect(parseCSVParam("new,contacted,won")).toEqual(["new", "contacted", "won"]);
  });

  it("returns an empty array for null/empty, not [\"\"]", () => {
    // A naive "".split(",") returns [""], which would make every
    // isActive-style check think exactly one (bogus) filter is active.
    expect(parseCSVParam(null)).toEqual([]);
    expect(parseCSVParam("")).toEqual([]);
  });
});

describe("toggleCSVValue", () => {
  it("adds a value not yet present", () => {
    expect(toggleCSVValue(["new"], "won")).toEqual(["new", "won"]);
  });

  it("removes a value already present, preserving the rest", () => {
    expect(toggleCSVValue(["new", "contacted", "won"], "contacted")).toEqual(["new", "won"]);
  });

  it("does not mutate the input array", () => {
    const original = ["new"];
    toggleCSVValue(original, "won");
    expect(original).toEqual(["new"]);
  });
});

describe("hasAnyLeadFilter", () => {
  const empty = {
    status: [],
    source: [],
    assignedTo: "",
    keyword: "",
    createdFrom: "",
    createdTo: "",
  };

  it("is false when nothing is set — the no-data empty state depends on this", () => {
    expect(hasAnyLeadFilter(empty)).toBe(false);
  });

  it("is true for exactly one filter set, whichever it is", () => {
    expect(hasAnyLeadFilter({ ...empty, status: ["won"] })).toBe(true);
    expect(hasAnyLeadFilter({ ...empty, source: ["manual"] })).toBe(true);
    expect(hasAnyLeadFilter({ ...empty, assignedTo: "none" })).toBe(true);
    expect(hasAnyLeadFilter({ ...empty, keyword: "budi" })).toBe(true);
    expect(hasAnyLeadFilter({ ...empty, createdFrom: "2026-01-01" })).toBe(true);
    expect(hasAnyLeadFilter({ ...empty, createdTo: "2026-01-31" })).toBe(true);
  });
});
