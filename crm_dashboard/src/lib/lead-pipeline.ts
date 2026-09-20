// What the status area of the lead detail screen shows, as pure functions.
// The component that renders it (leads/[id]/lead-status-panel.tsx) is
// deliberately thin: Vitest only loads *.test.ts, so everything that can be
// silently wrong — which step is current, which sections exist — lives here
// where it can be tested, the same split lib/nav.ts already uses (issue #141).
import { STATUS_META, type LeadStatus } from "./labels";
import {
  MAIN_PATH,
  statusTransitionOptions,
  type StatusDirection,
  type StatusTransitionOption,
} from "./lead-status";

// "inactive" is a closed lead (lost / unqualified / spam): it has no position
// on the main path, and marking a step as current would be inventing one.
export type PipelineStepState = "done" | "current" | "upcoming" | "inactive";

export interface PipelineStep {
  status: LeadStatus;
  label: string;
  state: PipelineStepState;
}

export function pipelineSteps(status: LeadStatus): PipelineStep[] {
  const current = MAIN_PATH.indexOf(status);
  return MAIN_PATH.map((step, index) => ({
    status: step,
    label: STATUS_META[step].label,
    state:
      current === -1 ? "inactive" : index < current ? "done" : index === current ? "current" : "upcoming",
  }));
}

export interface ActionGroup {
  key: StatusDirection;
  title: string;
  actions: StatusTransitionOption[];
}

// Fixed order, so the same section is always in the same place. Titles are
// verbs for the same reason the labels are: the section says WHAT the buttons
// under it do, instead of leaving the reader to infer it from an arrow.
const GROUPS: ReadonlyArray<{ key: StatusDirection; title: string }> = [
  { key: "forward", title: "Lanjutkan" },
  { key: "back", title: "Kembali" },
  { key: "reopen", title: "Buka kembali" },
  { key: "close", title: "Tutup lead" },
];

// Only sections that have something in them: a lead at "new" has no way back,
// and rendering a "Kembali" heading over nothing would suggest one.
export function actionGroups(status: LeadStatus): ActionGroup[] {
  const options = statusTransitionOptions(status);
  return GROUPS.map((group) => ({
    ...group,
    actions: options.filter((option) => option.direction === group.key),
  })).filter((group) => group.actions.length > 0);
}

// Derived from the rules, not restated: a status is final exactly when the
// rules offer nothing out of it (unqualified, spam).
export function isFinalStatus(status: LeadStatus): boolean {
  return statusTransitionOptions(status).length === 0;
}

// The banner for a lead that left the main path. null while it is still on it.
export function closedNote(status: LeadStatus): string | null {
  return MAIN_PATH.includes(status) ? null : `Lead ditutup: ${STATUS_META[status].label}`;
}
