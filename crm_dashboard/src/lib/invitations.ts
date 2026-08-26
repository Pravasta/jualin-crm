// Typed wrapper for /v1/invitations — shapes verified against
// crm_be/internal/invitation/handler_http.go.
import { apiFetch } from "./api-client";
import type { Role } from "./labels";

export interface Invitation {
  id: string;
  email: string;
  role: Role;
  expires_at: string;
  created_at: string;
}

export function listInvitations(signal?: AbortSignal): Promise<Invitation[]> {
  return apiFetch<Invitation[]>("/v1/invitations", { signal });
}

// role=owner is rejected server-side (ck_invitations_role CHECK,
// migrations/0003_crm_core.sql — only admin/manager/employee). The
// design's own invite dialog only offered Admin/Manager in its <select>,
// missing Employee entirely; that's corrected here rather than copied —
// Employee is a completely ordinary invite target (they get the mobile
// app in Phase 5, but the account has to exist before then).
export type InvitableRole = "admin" | "manager" | "employee";

export function createInvitation(email: string, role: InvitableRole): Promise<Invitation> {
  return apiFetch<Invitation>("/v1/invitations", { method: "POST", body: { email, role } });
}

export function revokeInvitation(id: string): Promise<void> {
  return apiFetch<void>(`/v1/invitations/${id}`, { method: "DELETE" });
}

export interface InvitationTokenInfo {
  organization_name: string;
  email: string;
  /** false = branch 1 (set a name+password); true = branch 2 (must already be logged in as this email). */
  user_exists: boolean;
}

// Public — no session required (crm_be mounts this outside authMW).
export function getInvitationTokenInfo(token: string): Promise<InvitationTokenInfo> {
  return apiFetch<InvitationTokenInfo>(`/v1/invitations/token/${encodeURIComponent(token)}`);
}

export interface AcceptInvitationResult {
  user_id: string;
  membership_id: string;
  organization_id: string;
}

// full_name/password only matter for branch 1 (new user) — branch 2
// (existing user) ignores them and instead requires the request to
// already be authenticated as the invited email (crm_be's
// acceptExistingUser); apiFetch's credentials:'include' carries
// whatever session cookie is already present, same as every other call.
export function acceptInvitation(input: {
  token: string;
  fullName?: string;
  password?: string;
}): Promise<AcceptInvitationResult> {
  return apiFetch<AcceptInvitationResult>("/v1/invitations/accept", {
    method: "POST",
    body: { token: input.token, full_name: input.fullName, password: input.password },
  });
}
