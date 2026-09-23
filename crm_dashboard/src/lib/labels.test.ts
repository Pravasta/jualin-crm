import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

import { LEAD_STATUSES, STATUS_META } from "./labels";

// Contrast is computed here, not trusted from the design (issue #159).
// The designer's printed ratios have been wrong twice already (#40, #70),
// and on this round the token sheet's "≥4.6:1 on its own tint" didn't hold
// at the tint the badges were using. This test is what stops the next
// color tweak from quietly dropping a pair under WCAG AA.

type Linear = [number, number, number];
type OKLCH = [number, number, number];

function oklchToLinear([L, C, h]: OKLCH): Linear {
  const a = C * Math.cos((h * Math.PI) / 180);
  const b = C * Math.sin((h * Math.PI) / 180);
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  const clamp = (x: number) => Math.min(1, Math.max(0, x));
  return [
    clamp(4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s),
    clamp(-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s),
    clamp(-0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s),
  ];
}

function linearToOklch([r, g, b]: Linear): OKLCH {
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s;
  const A = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
  const B = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
  return [L, Math.hypot(A, B), ((Math.atan2(B, A) * 180) / Math.PI + 360) % 360];
}

function hexToLinear(hex: string): Linear {
  const v = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  return v.map((x) => (x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4)) as Linear;
}

// `color-mix(in oklch, X, white P%)` — white has no chroma, so the mix
// moves lightness and chroma toward white and keeps X's hue.
function mixWithWhite(color: Linear, whiteShare: number): Linear {
  const [L, C, h] = linearToOklch(color);
  return oklchToLinear([L + (1 - L) * whiteShare, C * (1 - whiteShare), h]);
}

const luminance = ([r, g, b]: Linear) => 0.2126 * r + 0.7152 * g + 0.0722 * b;

function contrast(a: Linear, b: Linear): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const WHITE: Linear = [1, 1, 1];
const AA = 4.5;

describe("STATUS_META — dashboard status colors", () => {
  it.each(LEAD_STATUSES)("%s: text on the white badge reaches AA", (status) => {
    expect(contrast(hexToLinear(STATUS_META[status].color), WHITE)).toBeGreaterThanOrEqual(AA);
  });

  it.each(LEAD_STATUSES)("%s: text on its own tint reaches AA", (status) => {
    const meta = STATUS_META[status];
    const share = Number(/white (\d+)%/.exec(meta.background)?.[1]) / 100;
    expect(share).toBeGreaterThan(0);
    const color = hexToLinear(meta.color);
    expect(contrast(color, mixWithWhite(color, share))).toBeGreaterThanOrEqual(AA);
  });

  it("shape follows meaning: main path pill, outcome square, excluded dashed", () => {
    const shapes = Object.fromEntries(LEAD_STATUSES.map((s) => [s, STATUS_META[s].shape]));
    expect(shapes).toEqual({
      new: "pill",
      contacted: "pill",
      qualified: "pill",
      proposal: "pill",
      won: "square",
      lost: "square",
      unqualified: "dashed",
      spam: "dashed",
    });
  });

  it("Tidak Memenuhi Syarat and Spam share a shape, so they must not share a color", () => {
    expect(STATUS_META.unqualified.color).not.toBe(STATUS_META.spam.color);
  });
});

describe("globals.css — theme text pairs", () => {
  const css = readFileSync(path.resolve(import.meta.dirname, "../app/globals.css"), "utf8");
  const root = css.slice(css.indexOf(":root {"), css.indexOf(".dark {"));

  function token(name: string): Linear {
    const match = new RegExp(`--${name}:\\s*oklch\\(([\\d.]+) ([\\d.]+) ([\\d.]+)\\)`).exec(root);
    if (!match) throw new Error(`token --${name} not found as a plain oklch() in :root`);
    return oklchToLinear([Number(match[1]), Number(match[2]), Number(match[3])]);
  }

  it.each([
    ["foreground", "background"],
    ["foreground", "card"],
    ["foreground", "muted"],
    ["muted-foreground", "card"],
    ["muted-foreground", "background"],
    ["muted-foreground", "muted"],
    ["muted-foreground", "secondary"],
    ["secondary-foreground", "secondary"],
    ["primary-foreground", "primary"],
    ["accent-strong", "card"],
    ["accent-strong", "background"],
    ["card", "destructive"],
  ])("--%s on --%s reaches AA", (fg, bg) => {
    expect(contrast(token(fg), token(bg))).toBeGreaterThanOrEqual(AA);
  });
});
