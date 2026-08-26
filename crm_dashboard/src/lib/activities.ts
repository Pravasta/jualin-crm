// Typed wrapper for /v1/leads/{id}/activities — shape verified against
// crm_be/internal/activity/handler_http.go's activityJSON.
//
// Append-only on the backend (no PATCH/DELETE route exists at all — see
// that file's own comment) — this module mirrors that by never exposing
// an update or delete function, not just by convention.
import { apiFetch } from "./api-client";

// The 7 types crm_be writes automatically (ck_activities_type,
// migrations/0003_crm_core.sql) plus the 3 a human can create via
// createActivity below.
export type ActivityType =
  | "lead_created"
  | "lead_assigned"
  | "lead_unassigned"
  | "status_changed"
  | "lead_converted"
  | "note_added"
  | "call_logged"
  | "whatsapp_opened"
  | "task_created"
  | "task_completed";

export interface Activity {
  id: string;
  lead_id: string;
  type: ActivityType;
  actor_membership_id: string | null;
  body: string | null;
  metadata: Record<string, unknown> | null;
  created_at: string;
}

export function listActivities(leadId: string, signal?: AbortSignal): Promise<Activity[]> {
  return apiFetch<Activity[]>(`/v1/leads/${leadId}/activities`, { signal });
}

// Only these three — POST .../activities rejects any other type with
// 422 invalid_activity_type (the system-generated types are written
// internally by their own usecases, never through this endpoint).
export type UserActivityType = "note_added" | "call_logged" | "whatsapp_opened";

export function createActivity(
  leadId: string,
  type: UserActivityType,
  body: string
): Promise<Activity> {
  return apiFetch<Activity>(`/v1/leads/${leadId}/activities`, {
    method: "POST",
    body: { type, body },
  });
}
