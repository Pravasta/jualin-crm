// Which status transitions the UI offers. Mirrors
// crm_be/internal/lead/usecase.go's validateStatusTransition EXACTLY —
// this is NOT the Claude Design mockup's simplified TRANSITIONS map
// (which, e.g., only lets "lost" reopen to "new"). Issue #33's
// acceptance criterion is "transisi yang tidak sah tidak ditawarkan di
// UI"; a button that the backend then rejects is a worse failure than
// one the UI never shows.
import { STATUS_META, type LeadStatus } from "./labels";

const MAIN_PATH: LeadStatus[] = ["new", "contacted", "qualified", "proposal", "won"];
const SIDE_EXITS: LeadStatus[] = ["lost", "unqualified", "spam"];

function mainPathIndex(status: LeadStatus): number {
  return MAIN_PATH.indexOf(status);
}

// Line-for-line port of the Go function of the same name.
export function isValidStatusTransition(from: LeadStatus, to: LeadStatus): boolean {
  if (from === "unqualified" || from === "spam") return false;
  if (to === from) return to === "lost";
  if (to === "unqualified" || to === "spam" || to === "lost") return true;

  const toIdx = mainPathIndex(to);
  if (toIdx === -1) return false;
  if (from === "lost") return true;

  const fromIdx = mainPathIndex(from);
  if (fromIdx === -1) return false;
  const diff = toIdx - fromIdx;
  return diff === 1 || diff === -1;
}

export interface StatusTransitionOption {
  status: LeadStatus;
  label: string;
  /** "step" = adjacent on the main path (accent-styled); "exit" = a side terminal (neutral-styled). */
  kind: "step" | "exit";
}

// The backend allows "lost" to reopen to ANY main-path status (a
// documented simplification from TD phase 2 §5's ideal "one step back
// to whatever it was before" — crm_be issue #20's notes: not
// implementable without activity history). Offering all five as buttons
// would be a wall of options for a case that's actually rare; the
// design's own choice — reopen to "Baru" only — is the sensible default
// and is unconditionally valid per the rule above, so it's kept here
// rather than re-litigated.
export function statusTransitionOptions(from: LeadStatus): StatusTransitionOption[] {
  if (from === "lost") {
    return [
      {
        status: "new",
        label: `→ Buka kembali ke ${STATUS_META.new.label}`,
        kind: "step",
      },
    ];
  }

  const options: StatusTransitionOption[] = [];
  const idx = mainPathIndex(from);
  if (idx !== -1) {
    if (idx - 1 >= 0) {
      const prev = MAIN_PATH[idx - 1];
      options.push({ status: prev, label: `→ ${STATUS_META[prev].label}`, kind: "step" });
    }
    if (idx + 1 < MAIN_PATH.length) {
      const next = MAIN_PATH[idx + 1];
      options.push({ status: next, label: `→ ${STATUS_META[next].label}`, kind: "step" });
    }
  }

  for (const exit of SIDE_EXITS) {
    if (isValidStatusTransition(from, exit)) {
      options.push({ status: exit, label: STATUS_META[exit].label, kind: "exit" });
    }
  }
  return options;
}

// TD phase 3 §5 / issue #33: konversi hanya ditawarkan saat status
// "won" — checked here once so the button and any future call site
// agree, rather than each re-typing the string literal.
export function canConvertLead(status: LeadStatus): boolean {
  return status === "won";
}
