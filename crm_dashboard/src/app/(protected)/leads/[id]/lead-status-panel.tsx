"use client";

import { Fragment } from "react";
import { Check } from "lucide-react";
import { STATUS_META, type LeadStatus } from "@/lib/labels";
import {
  actionGroups,
  closedNote,
  isFinalStatus,
  pipelineSteps,
  type PipelineStep,
} from "@/lib/lead-pipeline";
import type { StatusDirection } from "@/lib/lead-status";
import { cn } from "@/lib/utils";

// The status area of the lead detail screen (issue #141): where the lead is
// in the pipeline, and what can be done next — grouped by what each button
// DOES. All the decisions (which step is current, which sections exist) live
// in lib/lead-pipeline.ts, where Vitest can reach them; this file only draws.
//
// Departs from the Claude Design output for this row, at the product owner's
// request: one arrow for both directions read as forward everywhere.

interface LeadStatusPanelProps {
  status: LeadStatus;
  /** A converted lead's status is locked (issue #142, ADR-016). */
  converted: boolean;
  saving: boolean;
  onChoose: (status: LeadStatus) => void;
}

export function LeadStatusPanel({ status, converted, saving, onChoose }: LeadStatusPanelProps) {
  const note = closedNote(status);
  const groups = converted ? [] : actionGroups(status);

  return (
    <div className="mt-4 flex flex-col gap-4 border-t border-border pt-4">
      <div>
        <div className="mb-2.5 text-xs font-medium text-muted-foreground">Tahapan lead</div>
        <Stepper status={status} />
      </div>

      {note && (
        <div
          className="rounded-md px-2.5 py-1.5 text-[13px] font-medium"
          style={{ background: STATUS_META[status].background, color: STATUS_META[status].color }}
        >
          {note}
        </div>
      )}

      {converted ? (
        // Not a disabled row of buttons: there is nothing left to choose, so
        // the screen says why instead (issue #142).
        <div className="text-[13px] text-muted-foreground">
          Lead ini sudah dikonversi menjadi Customer; statusnya tidak dapat diubah lagi.
        </div>
      ) : isFinalStatus(status) ? (
        <div className="text-[13px] text-muted-foreground">Status ini bersifat final.</div>
      ) : (
        <div className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-[104px_1fr] sm:items-center sm:gap-y-2.5">
          {groups.map((group) => (
            <Fragment key={group.key}>
              <div className="text-[12.5px] font-semibold text-muted-foreground">{group.title}</div>
              <div className="flex flex-wrap gap-2">
                {group.actions.map((action) => (
                  <button
                    key={action.status}
                    type="button"
                    disabled={saving}
                    onClick={() => onChoose(action.status)}
                    className={cn(
                      "min-h-11 rounded-lg border-[1.5px] px-3.5 text-[14px] font-semibold disabled:opacity-50 md:min-h-8 md:px-3 md:text-[13px]",
                      BUTTON_STYLE[action.direction]
                    )}
                  >
                    {action.label}
                  </button>
                ))}
              </div>
            </Fragment>
          ))}
        </div>
      )}
    </div>
  );
}

// Moving the lead along (forward, reopen) is the accent; stepping back and
// closing are neutral. Tokens, not raw colors — the row this replaces carried
// its own oklch() literals.
const BUTTON_STYLE: Record<StatusDirection, string> = {
  forward: "border-primary bg-primary text-primary-foreground hover:bg-primary/90",
  reopen: "border-primary bg-primary text-primary-foreground hover:bg-primary/90",
  back: "border-input bg-card text-foreground hover:bg-muted",
  close: "border-input bg-card text-foreground hover:bg-muted",
};

const MARKER = 22; // px — marker diameter; the connector is centred on it

function Stepper({ status }: { status: LeadStatus }) {
  const steps = pipelineSteps(status);
  const currentColor = STATUS_META[status].color;

  return (
    <ol aria-label="Tahapan lead" className="flex">
      {steps.map((step, index) => (
        <li
          key={step.status}
          aria-current={step.state === "current" ? "step" : undefined}
          className="relative flex min-w-0 flex-1 flex-col items-center gap-1.5 text-center"
        >
          {index > 0 && (
            // The segment between the previous marker and this one: filled
            // once this stage has been reached. z-0 so the marker sits over it.
            <span
              aria-hidden
              className={cn(
                "absolute right-1/2 h-0.5 w-full",
                step.state === "done" || step.state === "current" ? "bg-primary" : "bg-border"
              )}
              style={{ top: MARKER / 2 - 1 }}
            />
          )}
          <Marker state={step.state} color={currentColor} />
          <span
            className={cn(
              // 10.5px below 640px: at 360px each of the five steps gets ~60px,
              // and "Penawaran" in semibold at 11.5px is ~62px — one word can't
              // wrap, so it spilled into its neighbour (#162). overflow-wrap:
              // anywhere is the last-resort guard: "wrap, never overflow"
              // (design brief §9.2).
              "w-full px-0.5 text-[10.5px] leading-tight [overflow-wrap:anywhere] sm:text-[11.5px]",
              step.state === "current" ? "font-semibold text-foreground" : "",
              step.state === "done" ? "text-foreground" : "",
              step.state === "upcoming" || step.state === "inactive" ? "text-muted-foreground" : ""
            )}
          >
            {step.label}
            {step.state === "done" && <span className="sr-only"> (selesai)</span>}
          </span>
          {step.state === "current" && (
            // A word, not just a colour — position must not depend on hue.
            <span className="-mt-1 text-[11px] font-medium" style={{ color: currentColor }}>
              Saat ini
            </span>
          )}
        </li>
      ))}
    </ol>
  );
}

// Three shapes, so the state survives without colour: a check (done), a ring
// with a dot (current), an empty ring (not reached / lead closed).
function Marker({ state, color }: { state: PipelineStep["state"]; color: string }) {
  const size = { width: MARKER, height: MARKER };

  if (state === "done") {
    return (
      <span
        className="relative z-10 flex items-center justify-center rounded-full bg-primary text-primary-foreground"
        style={size}
      >
        <Check className="size-3.5" aria-hidden strokeWidth={3} />
      </span>
    );
  }
  if (state === "current") {
    return (
      <span
        className="relative z-10 flex items-center justify-center rounded-full border-2 bg-card"
        style={{ ...size, borderColor: color }}
      >
        <span className="size-2.5 rounded-full" style={{ background: color }} />
      </span>
    );
  }
  return (
    <span
      className={cn(
        "relative z-10 rounded-full border-2 border-border",
        state === "inactive" ? "bg-muted" : "bg-card"
      )}
      style={size}
    />
  );
}
