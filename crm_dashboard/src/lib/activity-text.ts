// Turns one Activity into what the timeline actually shows. Split out as
// a pure function (no React) so every metadata shape crm_be's usecases
// write — verified against internal/{lead,task,customer}/usecase.go's
// Activity.Record calls, not guessed — has a locked, tested rendering.
import type { Activity } from "./activities";
import { LOST_REASON_LABELS, STATUS_META, type LeadStatus, type LostReason } from "./labels";

export interface TimelineEntry {
  /** true for the 3 user-authored types (note_added/call_logged/whatsapp_opened). */
  isHuman: boolean;
  text: string;
  /** Only set for isHuman entries — resolved from actor_membership_id. */
  authorName?: string;
}

function memberName(id: string | null, namesById: Map<string, string>): string {
  if (!id) return "Seseorang";
  return namesById.get(id) ?? "Anggota yang sudah tidak aktif";
}

// namesById maps membership_id -> full_name (built from GET
// /v1/memberships, same map @/app/(protected)/leads's list screen
// already builds — see notes.md "## #33" for why a membership missing
// from that map isn't an error: it can be a deactivated member whose
// name the org still needs to see in old history).
export function activityToTimelineEntry(
  activity: Activity,
  namesById: Map<string, string>
): TimelineEntry {
  const meta = (activity.metadata ?? {}) as Record<string, unknown>;
  const str = (key: string): string | null =>
    typeof meta[key] === "string" && meta[key] !== "" ? (meta[key] as string) : null;

  switch (activity.type) {
    case "lead_created":
      // metadata is nil for this type (internal/lead/usecase.go's
      // Create) — the source is already shown in the header, so this
      // stays a plain, static line rather than plumbing lead.source
      // through just for this one sentence.
      return { isHuman: false, text: "Lead dibuat" };

    case "status_changed": {
      const from = str("from") as LeadStatus | null;
      const to = str("to") as LeadStatus | null;
      const fromLabel = from ? STATUS_META[from].label : "?";
      const toLabel = to ? STATUS_META[to].label : "?";
      return { isHuman: false, text: `Status: ${fromLabel} → ${toLabel}` };
    }

    case "lead_assigned": {
      const from = str("from");
      const to = str("to");
      const toName = memberName(to, namesById);
      return {
        isHuman: false,
        text: from
          ? `Dipindahkan dari ${memberName(from, namesById)} ke ${toName}`
          : `Ditugaskan ke ${toName}`,
      };
    }

    case "lead_unassigned":
      return { isHuman: false, text: `Dilepas dari ${memberName(str("from"), namesById)}` };

    case "lead_converted":
      return { isHuman: false, text: "Dikonversi menjadi customer" };

    case "task_created": {
      const title = str("title");
      return { isHuman: false, text: title ? `Task dibuat: ${title}` : "Task dibuat" };
    }

    case "task_completed":
      return { isHuman: false, text: "Task diselesaikan" };

    case "note_added":
      return {
        isHuman: true,
        text: activity.body ?? "",
        authorName: memberName(activity.actor_membership_id, namesById),
      };
    case "call_logged":
      return {
        isHuman: true,
        text: activity.body ?? "",
        authorName: memberName(activity.actor_membership_id, namesById),
      };
    case "whatsapp_opened":
      return {
        isHuman: true,
        text: activity.body ?? "",
        authorName: memberName(activity.actor_membership_id, namesById),
      };
  }
}

export function lostReasonDisplayLabel(reason: LostReason | null): string | null {
  return reason ? LOST_REASON_LABELS[reason] : null;
}
