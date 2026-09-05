import { describe, expect, it } from "vitest";
import { initialsOf, isActive, NAV_ITEMS, pageTitle } from "./nav";

describe("isActive", () => {
  it('matches "/" only exactly — otherwise every page highlights Beranda', () => {
    expect(isActive("/", "/")).toBe(true);
    expect(isActive("/leads", "/")).toBe(false);
    expect(isActive("/settings", "/")).toBe(false);
  });

  it("keeps the section active on its detail pages", () => {
    expect(isActive("/leads/01931f2e-abcd", "/leads")).toBe(true);
    expect(isActive("/leads", "/leads")).toBe(true);
  });

  it("does not match a different section that merely shares a prefix", () => {
    // Guards the naive startsWith: "/team" must not light up for
    // "/teams-report" if such a route is ever added.
    expect(isActive("/teams-report", "/team")).toBe(false);
    expect(isActive("/customers", "/customer")).toBe(false);
    // Same guard for /connect (issue #86) — td.md §10 calls this out by
    // name as the collision to watch for.
    expect(isActive("/connecting-something", "/connect")).toBe(false);
  });

  it("keeps Connect active on its sub-pages", () => {
    expect(isActive("/connect", "/connect")).toBe(true);
    expect(isActive("/connect/api", "/connect")).toBe(true);
    expect(isActive("/connect/api/docs", "/connect")).toBe(true);
  });
});

describe("pageTitle", () => {
  it("picks the most specific section, not the first that matches", () => {
    expect(pageTitle("/")).toBe("Beranda");
    expect(pageTitle("/leads")).toBe("Lead");
    expect(pageTitle("/leads/01931f2e-abcd")).toBe("Lead");
    expect(pageTitle("/team")).toBe("Tim");
    expect(pageTitle("/connect")).toBe("Connect");
    // /connect is a prefix of /connect/api — href-length sort must still
    // pick "Connect" (there's no more specific NAV_ITEMS entry for
    // sub-pages, but this proves the sort doesn't accidentally pick a
    // shorter, unrelated href instead).
    expect(pageTitle("/connect/api")).toBe("Connect");
    expect(pageTitle("/subscription")).toBe("Langganan");
    expect(pageTitle("/settings")).toBe("Pengaturan");
  });

  it("falls back for an unknown path rather than rendering empty", () => {
    expect(pageTitle("/tidak-ada")).toBe("Jualin CRM");
  });
});

describe("NAV_ITEMS", () => {
  // Acceptance criterion #12 — the interface is Indonesian throughout.
  // "Lead" and "Customer" are the exceptions glossary.md fixes as the
  // product's own vocabulary; the design's "Home"/"Task"/"Settings" are
  // not, and must not creep back in.
  it("uses Indonesian labels except for the two glossary terms", () => {
    const labels = NAV_ITEMS.map((item) => item.label);
    expect(labels).toEqual(["Beranda", "Lead", "Customer", "Tugas", "Tim", "Connect", "Langganan", "Pengaturan"]);
    expect(labels).not.toContain("Home");
    expect(labels).not.toContain("Task");
    expect(labels).not.toContain("Settings");
  });
});

describe("initialsOf", () => {
  it("takes first and last initial", () => {
    expect(initialsOf("Budi Santoso")).toBe("BS");
    expect(initialsOf("Budi Rahmat Santoso")).toBe("BS");
  });

  it("handles a single name and messy whitespace", () => {
    expect(initialsOf("Budi")).toBe("BU");
    expect(initialsOf("  Budi   Santoso  ")).toBe("BS");
  });

  it("never renders empty for an empty name", () => {
    expect(initialsOf("")).toBe("?");
    expect(initialsOf("   ")).toBe("?");
  });
});
