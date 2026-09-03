// Typed wrapper for /v1/webhook-endpoints and /v1/webhook-deliveries —
// shapes verified against crm_be/internal/webhook/handler_http.go's
// endpointJSON and deliveryJSON.
//
// Endpoints are the FOURTH credential this product issues, and the first
// whose trust direction is reversed: the other three prove someone else
// to us, this one proves US to someone else (ADR-005 lineage, Phase 7
// td.md §2). That difference is why the secret has to be reproducible on
// the server — but it changes nothing here: the raw secret still leaves
// the API exactly once, on create, and this file is where that is
// enforced in the type system.
//
// The endpoint list is NOT paginated (a plain array via httpx.OK, same as
// /v1/forms and /v1/api-keys); the delivery history IS (httpx.List with
// meta.total), so it uses apiFetchList.
import { apiFetch, apiFetchList } from "./api-client";
import { ApiError, type Meta } from "./api-types";

// The closed set of events crm_be emits (webhook.KnownEvents). A union,
// not `string`, so subscribing to an event the backend has never heard of
// is a compile error rather than a silently dead subscription.
export const WEBHOOK_EVENTS = ["lead.created", "lead.status_changed"] as const;

export type WebhookEvent = (typeof WEBHOOK_EVENTS)[number];

// What each event means to someone choosing which to subscribe to —
// phrased as when it fires, not as a restatement of its name.
export const WEBHOOK_EVENT_LABELS: Record<WebhookEvent, string> = {
  "lead.created": "Lead baru masuk",
  "lead.status_changed": "Status lead berubah",
};

export const WEBHOOK_EVENT_DESCRIPTIONS: Record<WebhookEvent, string> = {
  "lead.created": "Dikirim setiap kali lead baru tercatat, dari sumber mana pun.",
  "lead.status_changed": "Dikirim saat lead berpindah tahap, misalnya dari Baru ke Dihubungi.",
};

export interface WebhookEndpoint {
  id: string;
  url: string;
  secret_prefix: string;
  events: WebhookEvent[];
  description: string;
  is_active: boolean;
  created_by_membership_id: string | null;
  created_at: string;
  updated_at: string;
}

// secret does NOT exist on WebhookEndpoint — only here, and only
// createWebhookEndpoint returns this type. Rendering a secret from the
// LIST is a type error, not a runtime bug someone has to notice (Aturan
// #21). Exactly the CreatedAPIKey shape from #48, for the same reason.
export interface CreatedWebhookEndpoint extends WebhookEndpoint {
  secret: string;
}

export type DeliveryStatus = "pending" | "delivering" | "succeeded" | "failed";

export interface WebhookDelivery {
  id: string;
  endpoint_id: string;
  event_type: string;
  // The frozen event snapshot (td.md §1.1). Rendered read-only, never
  // parsed for meaning — `unknown` rather than a shaped type so nothing
  // here grows a second definition of a lead.
  payload: unknown;
  status: DeliveryStatus;
  attempt: number;
  next_attempt_at: string;
  response_status: number | null;
  error: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateWebhookEndpointInput {
  url: string;
  events: WebhookEvent[];
  description?: string;
}

// Every field optional — the backend treats absent as "leave unchanged"
// (webhook.UpdateInput's nil convention), so sending only what changed is
// the correct PATCH, not a partial-update shortcut.
//
// There is no `version` here on purpose: endpointJSON carries none, and
// the backend's PATCH is last-write-wins. Aturan #35's optimistic locking
// binds leads and tasks, not this.
export interface UpdateWebhookEndpointInput {
  url?: string;
  events?: WebhookEvent[];
  description?: string;
  is_active?: boolean;
}

export function listWebhookEndpoints(signal?: AbortSignal): Promise<WebhookEndpoint[]> {
  return apiFetch<WebhookEndpoint[]>("/v1/webhook-endpoints", { signal });
}

export function getWebhookEndpoint(id: string, signal?: AbortSignal): Promise<WebhookEndpoint> {
  return apiFetch<WebhookEndpoint>(`/v1/webhook-endpoints/${id}`, { signal });
}

export function createWebhookEndpoint(
  input: CreateWebhookEndpointInput
): Promise<CreatedWebhookEndpoint> {
  return apiFetch<CreatedWebhookEndpoint>("/v1/webhook-endpoints", {
    method: "POST",
    body: input,
  });
}

export function updateWebhookEndpoint(
  id: string,
  input: UpdateWebhookEndpointInput
): Promise<WebhookEndpoint> {
  return apiFetch<WebhookEndpoint>(`/v1/webhook-endpoints/${id}`, {
    method: "PATCH",
    body: input,
  });
}

// Soft delete (deleted_at), the forms pattern rather than api_keys'
// revoked_at — the endpoint disappears from the list rather than
// lingering as a revoked row. Not idempotent: a second call gets 404.
export function deleteWebhookEndpoint(id: string): Promise<void> {
  return apiFetch<void>(`/v1/webhook-endpoints/${id}`, { method: "DELETE" });
}

export function listWebhookDeliveries(
  endpointId: string,
  page: number,
  signal?: AbortSignal
): Promise<{ data: WebhookDelivery[]; meta: Meta }> {
  return apiFetchList<WebhookDelivery>(
    `/v1/webhook-endpoints/${endpointId}/deliveries?page=${page}`,
    { signal }
  );
}

// Only valid for a `failed` delivery — anything else comes back 409
// delivery_not_retryable, which the caller must surface rather than
// swallow (issue #103 AC: "bukan tombol yang diam"). The backend checks
// again under its own WHERE, so this is not a race the UI can lose
// dangerously; it just means the button can legitimately fail.
export function retryWebhookDelivery(deliveryId: string): Promise<WebhookDelivery> {
  return apiFetch<WebhookDelivery>(`/v1/webhook-deliveries/${deliveryId}/retry`, {
    method: "POST",
  });
}

// webhook_url_not_allowed is a plain 400 (td.md §7), NOT validation_failed
// — it carries no details[] to place it under a field, but it is always
// about the URL. Both the create dialog and the editor use this to steer
// the message under the URL input instead of into a banner that leaves
// the offending field unmarked.
//
// Same shape as auth-errors.ts's isLeadAlreadyConverted, and lives here
// rather than in either component so the two cannot drift.
//
// The message itself is always shown as-is. Its vagueness is deliberate:
// distinguishing "private address" from "cannot resolve" would hand a
// customer a way to map our internal network through error messages.
export function isWebhookUrlNotAllowed(err: unknown): boolean {
  return err instanceof ApiError && err.code === "webhook_url_not_allowed";
}

// The create dialog's close guard, as a pure function.
//
// Aturan #21 says the secret is shown exactly once — which means the
// dialog must not be dismissable while it is on screen and unacknowledged,
// through ANY route: the X button, Escape, or a backdrop click. All three
// arrive at base-ui's onOpenChange, so the guard lives at that one choke
// point rather than on a disabled button (which stops only the button).
//
// Extracted here rather than left inline because the alternative way to
// prove it is a browser, and this codebase deliberately keeps visual
// testing out of scope (TD phase 3 §9). The same reasoning that put
// canManageWebhooks and lib/nav.ts in their own files: a rule worth
// getting right is worth being able to test without rendering React.
export function canCloseCreateDialog(step: "form" | "reveal", confirmedSaved: boolean): boolean {
  if (step !== "reveal") return true;
  return confirmedSaved;
}

