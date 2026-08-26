// Backend enum values → what the user actually reads. One place, so the
// same status can't end up as "Menang" on one screen and "Won" on
// another (Aturan #12: seluruh teks antarmuka Bahasa Indonesia).
//
// Labels and colors come from the Claude Design output (issue #40). The
// colors are NOT CSS tokens: each one belongs to exactly one enum value
// rather than to the theme, and Tailwind can't generate classes from a
// runtime value anyway — these are applied as inline styles by whatever
// renders the badge.

// --- Lead status -------------------------------------------------------

export const LEAD_STATUSES = [
  "new",
  "contacted",
  "qualified",
  "proposal",
  "won",
  "lost",
  "unqualified",
  "spam",
] as const;

export type LeadStatus = (typeof LEAD_STATUSES)[number];

export interface StatusMeta {
  label: string;
  /** Badge text color. Also the dot/border color where a badge isn't used. */
  color: string;
  /** Badge background — the same hue at 15% over white. */
  background: string;
}

// Every `color` below is verified to reach at least 4.5:1 (WCAG AA,
// normal text) against its own `background`. The design's original
// values missed that bar on five of the eight — `proposal` worst at
// 3.14:1 — so lightness is nudged down while hue and chroma are kept
// exactly as designed. Backgrounds are still mixed from the design's
// original lightness, so the badges look as drawn.
export const STATUS_META: Record<LeadStatus, StatusMeta> = {
  new: {
    label: "Baru",
    color: "oklch(0.52 0.18 255)",
    background: "color-mix(in oklch, oklch(0.55 0.18 255), white 85%)",
  },
  contacted: {
    label: "Dihubungi",
    color: "oklch(0.535 0.16 300)",
    background: "color-mix(in oklch, oklch(0.55 0.16 300), white 85%)",
  },
  qualified: {
    label: "Memenuhi Syarat",
    color: "oklch(0.49 0.12 195)",
    background: "color-mix(in oklch, oklch(0.5 0.12 195), white 85%)",
  },
  proposal: {
    label: "Penawaran",
    color: "oklch(0.53 0.15 75)",
    background: "color-mix(in oklch, oklch(0.62 0.15 75), white 85%)",
  },
  won: {
    label: "Menang",
    color: "oklch(0.5 0.15 145)",
    background: "color-mix(in oklch, oklch(0.5 0.15 145), white 85%)",
  },
  lost: {
    label: "Kalah",
    color: "oklch(0.54 0.2 25)",
    background: "color-mix(in oklch, oklch(0.55 0.2 25), white 85%)",
  },
  unqualified: {
    label: "Tidak Memenuhi Syarat",
    color: "oklch(0.5 0 0)",
    background: "color-mix(in oklch, oklch(0.5 0 0), white 85%)",
  },
  spam: {
    label: "Spam",
    color: "oklch(0.42 0.03 30)",
    background: "color-mix(in oklch, oklch(0.42 0.03 30), white 85%)",
  },
};

// --- Lost reason -------------------------------------------------------

export const LOST_REASONS = [
  "price",
  "competitor",
  "timing",
  "no_response",
  "not_interested",
  "other",
] as const;

export type LostReason = (typeof LOST_REASONS)[number];

export const LOST_REASON_LABELS: Record<LostReason, string> = {
  price: "Harga",
  competitor: "Kompetitor",
  timing: "Waktu Tidak Tepat",
  no_response: "Tidak Merespons",
  not_interested: "Tidak Tertarik",
  other: "Lainnya",
};

// --- Lead source -------------------------------------------------------

export const LEAD_SOURCES = ["manual", "api", "form", "webhook"] as const;

export type LeadSource = (typeof LEAD_SOURCES)[number];

// "Formulir", not "Form" — this is the one source with a natural
// Indonesian word. API and Webhook stay as-is; they're proper nouns to
// the integrator who sees them.
export const SOURCE_LABELS: Record<LeadSource, string> = {
  manual: "Manual",
  api: "API",
  form: "Formulir",
  webhook: "Webhook",
};

// --- Role --------------------------------------------------------------

export const ROLES = ["owner", "admin", "manager", "employee"] as const;

export type Role = (typeof ROLES)[number];

// Left in English on purpose: these are the exact terms glossary.md
// fixes for the role enum, and the product speaks about "Owner" and
// "Admin" that way throughout.
export const ROLE_LABELS: Record<Role, string> = {
  owner: "Owner",
  admin: "Admin",
  manager: "Manager",
  employee: "Employee",
};
