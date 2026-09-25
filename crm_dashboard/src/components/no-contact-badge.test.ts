import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NoContactBadge } from "./no-contact-badge";

describe("NoContactBadge", () => {
  it("says it in words — the marker must not depend on its colour", () => {
    expect(renderToStaticMarkup(createElement(NoContactBadge))).toContain("Belum ada kontak");
  });

  it("is neutral, not an error colour: a lead without contact is a legitimate state", () => {
    const html = renderToStaticMarkup(createElement(NoContactBadge));
    // A neutral surface token — `bg-secondary` since the Phase 8.6 tokens
    // (#161), `bg-muted` before. What matters is that it is one of the
    // neutrals, never a status or error color.
    expect(html).toMatch(/bg-(secondary|muted)\b/);
    expect(html).not.toMatch(/danger|destructive|red|amber/);
  });
});
