import { describe, expect, it } from "vitest";
import { activityToTimelineEntry } from "./activity-text";
import type { Activity } from "./activities";

const NAMES = new Map([
  ["m-budi", "Budi Santoso"],
  ["m-sari", "Sari Wulandari"],
]);

function activity(overrides: Partial<Activity>): Activity {
  return {
    id: "a-1",
    lead_id: "l-1",
    type: "note_added",
    actor_membership_id: null,
    body: null,
    metadata: null,
    created_at: "2026-08-17T09:00:00Z",
    ...overrides,
  };
}

describe("activityToTimelineEntry", () => {
  // lead_created: internal/lead/usecase.go's Create passes metadata=nil.
  it("lead_created — no metadata, static text", () => {
    const entry = activityToTimelineEntry(activity({ type: "lead_created", metadata: null }), NAMES);
    expect(entry).toEqual({ isHuman: false, text: "Lead dibuat" });
  });

  // status_changed: {"from": "...", "to": "..."} — internal/lead/usecase.go UpdateStatus.
  it("status_changed — renders from -> to using status labels, not raw enum values", () => {
    const entry = activityToTimelineEntry(
      activity({ type: "status_changed", metadata: { from: "new", to: "contacted" } }),
      NAMES
    );
    expect(entry.text).toBe("Status: Baru → Dihubungi");
  });

  // lead_assigned: {"from": <id|null>, "to": <id>} — internal/lead/usecase.go UpdateAssignment.
  it("lead_assigned — first-time assignment (no `from`) reads as \"ditugaskan ke\"", () => {
    const entry = activityToTimelineEntry(
      activity({ type: "lead_assigned", metadata: { to: "m-budi" } }),
      NAMES
    );
    expect(entry.text).toBe("Ditugaskan ke Budi Santoso");
  });

  it("lead_assigned — reassignment (both from and to present) reads as a transfer", () => {
    const entry = activityToTimelineEntry(
      activity({ type: "lead_assigned", metadata: { from: "m-budi", to: "m-sari" } }),
      NAMES
    );
    expect(entry.text).toBe("Dipindahkan dari Budi Santoso ke Sari Wulandari");
  });

  // lead_unassigned: {"from": <id>} — internal/lead/usecase.go UpdateAssignment.
  it("lead_unassigned", () => {
    const entry = activityToTimelineEntry(
      activity({ type: "lead_unassigned", metadata: { from: "m-budi" } }),
      NAMES
    );
    expect(entry.text).toBe("Dilepas dari Budi Santoso");
  });

  it("resolves a membership id no longer in the map as a deactivated member, not a crash", () => {
    const entry = activityToTimelineEntry(
      activity({ type: "lead_assigned", metadata: { to: "m-gone" } }),
      NAMES
    );
    expect(entry.text).toBe("Ditugaskan ke Anggota yang sudah tidak aktif");
  });

  // lead_converted: {"customer_id": "..."} — internal/customer/usecase.go Convert.
  it("lead_converted", () => {
    const entry = activityToTimelineEntry(
      activity({ type: "lead_converted", metadata: { customer_id: "c-1" } }),
      NAMES
    );
    expect(entry.text).toBe("Dikonversi menjadi customer");
  });

  // task_created: {"task_id": "...", "title": "..."} — internal/task/usecase.go Create.
  it("task_created — includes the title", () => {
    const entry = activityToTimelineEntry(
      activity({ type: "task_created", metadata: { task_id: "t-1", title: "Telepon lagi besok" } }),
      NAMES
    );
    expect(entry.text).toBe("Task dibuat: Telepon lagi besok");
  });

  // task_completed: {"task_id": "..."} ONLY — no title in this metadata,
  // unlike task_created. A naive implementation might assume it's there.
  it("task_completed — has no title in its metadata, must not crash or show \"undefined\"", () => {
    const entry = activityToTimelineEntry(
      activity({ type: "task_completed", metadata: { task_id: "t-1" } }),
      NAMES
    );
    expect(entry.text).toBe("Task diselesaikan");
  });

  it.each(["note_added", "call_logged", "whatsapp_opened"] as const)(
    "%s — human-authored, body is the text, author resolved from actor_membership_id",
    (type) => {
      const entry = activityToTimelineEntry(
        activity({ type, body: "Sudah dihubungi, tertarik.", actor_membership_id: "m-sari" }),
        NAMES
      );
      expect(entry.isHuman).toBe(true);
      expect(entry.text).toBe("Sudah dihubungi, tertarik.");
      expect(entry.authorName).toBe("Sari Wulandari");
    }
  );
});
