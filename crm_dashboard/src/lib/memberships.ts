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
