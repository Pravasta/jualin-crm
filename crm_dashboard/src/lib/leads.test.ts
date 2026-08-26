import { describe, expect, it } from "vitest";
import { buildQuery } from "./leads";

describe("buildQuery", () => {
  it("returns empty string for no filters — never a bare '?'", () => {
    expect(buildQuery({})).toBe("");
  });

  it("joins multi-select filters as comma-separated values, matching crm_be's splitCSV", () => {
    const qs = buildQuery({ status: ["new", "contacted"], source: ["manual", "form"] });
    const params = new URLSearchParams(qs.slice(1));
    expect(params.get("status")).toBe("new,contacted");
    expect(params.get("source")).toBe("manual,form");
  });

  it('passes assignedTo "none" through unchanged — the permanent safety-net filter', () => {
    const qs = buildQuery({ assignedTo: "none" });
    expect(new URLSearchParams(qs.slice(1)).get("assigned_to")).toBe("none");
  });

  it("omits every param the caller didn't set, rather than sending empty values", () => {
    const qs = buildQuery({ q: "budi" });
    const params = new URLSearchParams(qs.slice(1));
    expect(params.has("status")).toBe(false);
    expect(params.has("source")).toBe(false);
    expect(params.has("assigned_to")).toBe(false);
    expect(params.has("created_from")).toBe(false);
    expect(params.has("created_to")).toBe(false);
    expect(params.get("q")).toBe("budi");
  });

  it("carries page/per_page for pagination", () => {
    const qs = buildQuery({ page: 3, perPage: 25 });
    const params = new URLSearchParams(qs.slice(1));
    expect(params.get("page")).toBe("3");
    expect(params.get("per_page")).toBe("25");
  });
});
