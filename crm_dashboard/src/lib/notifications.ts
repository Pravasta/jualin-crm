// Typed wrapper for /v1/notifications — shapes verified against
// crm_be/internal/notification/handler_http.go's notificationJSON.
import { apiFetch } from "./api-client";

// ck_notifications_type, migrations/0004_notifications.sql — only these
// two exist through Phase 3.
export type NotificationType = "lead_assigned" | "task_assigned";

export interface Notification {
  id: string;
  type: NotificationType;
  lead_id: string | null;
  task_id: string | null;
  /** Already a complete Indonesian sentence (e.g. "Lead #1032 ditugaskan
   *  kepada Anda") — crm_be builds this string itself, shown as-is. */
  title: string;
  body: string | null;
  read_at: string | null;
  created_at: string;
}

export function listNotifications(
  opts: { unreadOnly?: boolean } = {},
  signal?: AbortSignal
): Promise<Notification[]> {
  const qs = opts.unreadOnly ? "?unread=true" : "";
  return apiFetch<Notification[]>(`/v1/notifications${qs}`, { signal });
}

export function markNotificationRead(id: string): Promise<void> {
  return apiFetch<void>(`/v1/notifications/${id}/read`, { method: "POST" });
}

export function markAllNotificationsRead(): Promise<void> {
  return apiFetch<void>("/v1/notifications/read-all", { method: "POST" });
}
