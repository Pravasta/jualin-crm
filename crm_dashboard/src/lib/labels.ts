// Backend enum values → what the user actually reads. One place, so the
// same status can't end up as "Menang" on one screen and "Won" on
// another (Aturan #12: seluruh teks antarmuka Bahasa Indonesia).
//
// Labels come from the Claude Design output (issue #40); colors and
// shapes from the Phase 8.6 token sheet (issue #159). The colors are NOT
// CSS tokens: each one belongs to exactly one enum value rather than to
// the theme, and Tailwind can't generate classes from a runtime value
// anyway — these are applied as inline styles by whatever renders them.

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

/**
 * How a status badge is drawn on the dashboard (token sheet "Opsi B").
 * The shape carries meaning so status never rests on color alone:
 *   pill   — the main path (Baru → Penawaran)
 *   square — an outcome (Menang, Kalah)
 *   dashed — excluded from conversion rate (Tidak Memenuhi Syarat, Spam)
 */
export type StatusShape = "pill" | "square" | "dashed";

export interface StatusMeta {
  label: string;
  /** Text and border color. Drawn on white in the badge itself. */
  color: string;
  /** Tint for surfaces that carry this status's text on a colored
   * background (closed-lead ribbon, active filter chip). */
  background: string;
  shape: StatusShape;
}

// Contrast, recomputed rather than copied (issue #159 — table in
// docs/phases/08.6-ui-redesign/notes.md): every `color` is 5.03–5.27:1
// on white, which is what the badge draws on, and the same for white
// text on `color` (an active chip).
//
// The token sheet claims these reach ≥4.6:1 "on their own tint". They
// don't at the old 85%-white tint (4.11–4.29:1), so `background` is
// mixed at 94% white instead: 4.65:1 at the worst (`won`). That is what
// AA needs for normal text wherever a status color sits on its tint.
const tint = (color: string) => `color-mix(in oklch, ${color}, white 94%)`;

const status = (label: string, color: string, shape: StatusShape): StatusMeta => ({
  label,
  color,
  background: tint(color),
  shape,
});

export const STATUS_META: Record<LeadStatus, StatusMeta> = {
  new: status("Baru", "#006cd3", "pill"),
  contacted: status("Dihubungi", "#8156c0", "pill"),
  qualified: status("Memenuhi Syarat", "#007b7d", "pill"),
  proposal: status("Penawaran", "#a05d00", "pill"),
  won: status("Menang", "#1d802b", "square"),
  lost: status("Kalah", "#ce2930", "square"),
  unqualified: status("Tidak Memenuhi Syarat", "#6d6d6d", "dashed"),
  spam: status("Spam", "#7f6964", "dashed"),
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

// --- API key scope -------------------------------------------------------

// Only one scope exists in this codebase today (ADR-004 aturan #4) — a
// Record over the full APIKeyScope union, not a partial map, so adding a
// second scope value without a label is a compile error, not a blank
// cell in the UI.
export const SCOPE_LABELS: Record<"leads:write", string> = {
  "leads:write": "Kirim lead",
};
