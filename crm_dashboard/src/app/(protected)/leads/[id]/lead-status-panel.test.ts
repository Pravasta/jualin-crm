import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { LeadStatus } from "@/lib/labels";
import { LeadStatusPanel } from "./lead-status-panel";

// Issue #141. Vitest cannot judge how the panel LOOKS — that is checked in a
// browser — but it can pin what the markup must contain: the accessibility
// attributes, which sections exist, and that no arrow survives. Rendered on
// the server (renderToStaticMarkup), so no DOM library is needed.

function render(status: LeadStatus, converted = false): string {
  return renderToStaticMarkup(
    createElement(LeadStatusPanel, { status, converted, saving: false, onChoose: () => {} })
  );
}

const count = (haystack: string, needle: string) => haystack.split(needle).length - 1;
const buttonLabels = (html: string) =>
  [...html.matchAll(/<button[^>]*>([^<]*)<\/button>/g)].map((m) => m[1]);

describe("LeadStatusPanel — a lead on the main path", () => {
  const html = render("proposal");

  it("marks exactly one step as current, with a word and not only a colour", () => {
    expect(count(html, 'aria-current="step"')).toBe(1);
    expect(count(html, "Saat ini")).toBe(1);
  });

  it("draws one check per stage already passed — the shape that survives without colour", () => {
    // Penawaran is the 4th stage: Baru, Dihubungi, Memenuhi Syarat are done.
    expect(count(html, "<svg")).toBe(3);
  });

  it("titles each section by what its buttons do, and labels them as verbs", () => {
    expect(html).toContain("Lanjutkan");
    expect(html).toContain("Kembali");
    expect(html).toContain("Tutup lead");
    expect(buttonLabels(html)).toEqual([
      "Maju ke Menang",
      "Kembali ke Memenuhi Syarat",
      "Kalah",
      "Tidak Memenuhi Syarat",
      "Spam",
    ]);
  });

  it("uses no arrow anywhere", () => {
    expect(html).not.toMatch(/[→←↑↓]/);
  });
});

describe("LeadStatusPanel — sections that would be empty are not rendered", () => {
  it("new: no Kembali section (there is no way back, ADR-015)", () => {
    const html = render("new");
    expect(html).not.toContain("Kembali");
    expect(html).toContain("Lanjutkan");
    expect(count(html, "<svg")).toBe(0); // nothing reached yet
  });

  it("won: no Lanjutkan section", () => {
    const html = render("won");
    expect(html).not.toContain("Lanjutkan");
    expect(html).toContain("Kembali");
    expect(buttonLabels(html)[0]).toBe("Kembali ke Penawaran");
  });
});

describe("LeadStatusPanel — a lead that left the main path", () => {
  it("lost: says it is closed, has no current step, and offers only reopening", () => {
    const html = render("lost");
    expect(html).toContain("Lead ditutup: Kalah");
    expect(html).not.toContain("aria-current");
    expect(html).not.toContain("Saat ini");
    expect(buttonLabels(html)).toEqual(["Buka kembali ke Dihubungi"]);
    expect(html).toContain("Buka kembali");
  });

  it.each(["unqualified", "spam"] as const)("%s: closed, final, and no buttons at all", (status) => {
    const html = render(status);
    expect(html).toContain("Lead ditutup:");
    expect(html).toContain("Status ini bersifat final.");
    expect(html).not.toContain("<button");
    expect(html).not.toContain("aria-current");
  });
});

describe("LeadStatusPanel — a converted lead (issue #142)", () => {
  it("keeps the stepper, drops every button, and says why", () => {
    const html = render("won", true);
    expect(html).toContain("Lead ini sudah dikonversi menjadi Customer; statusnya tidak dapat diubah lagi.");
    expect(html).not.toContain("<button");
    expect(count(html, 'aria-current="step"')).toBe(1);
  });
});
