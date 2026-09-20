// Which status transitions the UI offers. Mirrors
// crm_be/internal/lead/usecase.go's validateStatusTransition EXACTLY —
// this is NOT the Claude Design mockup's simplified TRANSITIONS map
// (which, e.g., only lets "lost" reopen to "new"). Issue #33's
// acceptance criterion is "transisi yang tidak sah tidak ditawarkan di
// UI"; a button that the backend then rejects is a worse failure than
// one the UI never shows.
import { STATUS_META, type LeadStatus } from "./labels";

// Exported so the pipeline view (lead-pipeline.ts) reads the SAME path the
// rules do — two copies of "the five stages" is how they drift apart.
export const MAIN_PATH: readonly LeadStatus[] = ["new", "contacted", "qualified", "proposal", "won"];
const SIDE_EXITS: readonly LeadStatus[] = ["lost", "unqualified", "spam"];

function mainPathIndex(status: LeadStatus): number {
  return MAIN_PATH.indexOf(status);
}

// Line-for-line port of the Go function of the same name.
export function isValidStatusTransition(from: LeadStatus, to: LeadStatus): boolean {
  if (from === "unqualified" || from === "spam") return false;
  if (to === from) return to === "lost";
  // Nothing leads back to "new" (issue #139, ADR-015): it means "belum
  // disentuh", which can never be true again once a lead has been touched.
  if (to === "new") return false;
  if (to === "unqualified" || to === "spam" || to === "lost") return true;

  const toIdx = mainPathIndex(to);
  if (toIdx === -1) return false;
  if (from === "lost") return true;

  const fromIdx = mainPathIndex(from);
  if (fromIdx === -1) return false;
  const diff = toIdx - fromIdx;
  return diff === 1 || diff === -1;
}

// Direction travels as DATA, and the label is a verb. Until #141 every step
// was labelled "→ {status}" — one arrow for both ways, so "→ Memenuhi Syarat"
// at Penawaran pointed forward while going backward. A verb carries the
// direction by itself, so there is no arrow left to read the wrong way.
export type StatusDirection = "forward" | "back" | "reopen" | "close";

export interface StatusTransitionOption {
  status: LeadStatus;
  label: string;
  direction: StatusDirection;
}

// The backend allows "lost" to reopen to ANY main-path status except
// "new" (a documented simplification from TD phase 2 §5's ideal "one step
// back to whatever it was before" — crm_be issue #20's notes: not
// implementable without activity history). Offering all four as buttons
// would be a wall of options for a case that's actually rare, so the UI
// offers one: "Dihubungi" — the earliest stage a reopened lead can
// honestly be in now that "Baru" is closed (ADR-015). It is valid per the
// rule above, and the "every option is valid" test keeps that true.
export function statusTransitionOptions(from: LeadStatus): StatusTransitionOption[] {
  if (from === "lost") {
    return [
      {
        status: "contacted",
        label: `Buka kembali ke ${STATUS_META.contacted.label}`,
        direction: "reopen",
      },
    ];
  }

  const options: StatusTransitionOption[] = [];
  const idx = mainPathIndex(from);
  if (idx !== -1) {
    // Neighbors are offered only if the rule says so — this is what keeps
    // "contacted" from showing a way back to "new". Deriving them from the
    // path index alone is how that button appeared in the first place.
    if (idx - 1 >= 0) {
      const prev = MAIN_PATH[idx - 1];
      if (isValidStatusTransition(from, prev)) {
        options.push({ status: prev, label: `Kembali ke ${STATUS_META[prev].label}`, direction: "back" });
      }
    }
    if (idx + 1 < MAIN_PATH.length) {
      const next = MAIN_PATH[idx + 1];
      if (isValidStatusTransition(from, next)) {
        options.push({ status: next, label: `Maju ke ${STATUS_META[next].label}`, direction: "forward" });
      }
    }
  }

  for (const exit of SIDE_EXITS) {
    if (isValidStatusTransition(from, exit)) {
      options.push({ status: exit, label: STATUS_META[exit].label, direction: "close" });
    }
  }
  return options;
}

// The Lead entity carries no "already converted" flag — converted_from_lead_id
// lives on Customer, and conversion never touches the lead (TD §12) — so the
// timeline's own "lead_converted" entry is the only signal the client has.
// Once it exists the backend locks the lead's status (issue #142, ADR-016);
// the screen uses this to stop OFFERING the buttons rather than offering them
// and letting the backend refuse. The backend stays the authority: a stale
// screen still gets lead_converted_locked, not a silent success.
export function hasBeenConverted(activities: ReadonlyArray<{ type: string }>): boolean {
  return activities.some((a) => a.type === "lead_converted");
}

// TD phase 3 §5 / issue #33: konversi hanya ditawarkan saat status
// "won" — checked here once so the button and any future call site
// agree, rather than each re-typing the string literal.
export function canConvertLead(status: LeadStatus): boolean {
  return status === "won";
}
