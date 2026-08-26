// Typed wrapper for GET /v1/memberships — used here to populate the
// owner filter and resolve assigned_to_membership_id to a display name.
// Not paginated (crm_be/internal/membership/handler_http.go returns a
// plain array), unlike /v1/leads.
import { apiFetch } from "./api-client";
import type { Role } from "./labels";

export interface Member {
  id: string;
  user_id: string;
  email: string;
  full_name: string;
  role: Role;
  created_at: string;
}

export function listMemberships(signal?: AbortSignal): Promise<Member[]> {
  return apiFetch<Member[]>("/v1/memberships", { signal });
}

export function updateMembershipRole(id: string, role: Role): Promise<void> {
  return apiFetch<void>(`/v1/memberships/${id}`, { method: "PATCH", body: { role } });
}

export interface DeactivateMembershipInput {
  /** "unassign" or "reassign" — omitted means the backend default "reject",
   *  which fails with 409 membership_has_open_leads if any lead is open. */
  onOpenLeads?: "unassign" | "reassign";
  /** Required when onOpenLeads is "reassign". */
  reassignTo?: string;
}

// The three-way branch (freeze 2.3 ketentuan #3 / issue #34's headline
// acceptance criterion) is NOT decided here — this function just sends
// whatever the caller already decided. The default call (no input) is
// what surfaces membership_has_open_leads in the first place; the
// caller is expected to try that first and only pass onOpenLeads once
// the user has consciously chosen unassign/reassign.
export function deactivateMembership(
  id: string,
  input: DeactivateMembershipInput = {}
): Promise<void> {
  const params = new URLSearchParams();
  if (input.onOpenLeads) params.set("on_open_leads", input.onOpenLeads);
  if (input.reassignTo) params.set("reassign_to", input.reassignTo);
  const qs = params.toString();
  return apiFetch<void>(`/v1/memberships/${id}${qs ? `?${qs}` : ""}`, { method: "DELETE" });
}
