// Pure logic for the API key management screen (#48) — split out so the
// coarse permission gate and the list's display shaping are testable
// without rendering React, same split as lib/nav.ts and
// lib/team-permissions.ts.
import { formatApproximateID } from "./date";
import { SCOPE_LABELS, type Role } from "./labels";
import type { APIKey } from "./api-keys";

// Owner/Admin only — Manager and Employee get NO access at all, not
// read-only (crm_be's authz.go: ActionAPIKeyCreate/List/Revoke are
// Owner/Admin only, unlike ActionMembershipList which Manager also
// holds). This is the coarse role gate; unlike team-permissions.ts there
// is no actor-vs-target relationship rule to layer on top of it — every
// Owner/Admin can manage every key in the organization equally.
export function canManageAPIKeys(role: Role): boolean {
  return role === "owner" || role === "admin";
}

export interface APIKeyRow {
  id: string;
  keyPrefix: string;
  name: string;
  scopeLabels: string;
  isRevoked: boolean;
  statusLabel: "Aktif" | "Dicabut";
  lastUsedLabel: string;
}

// toAPIKeyRow never touches `secret` — APIKey (unlike CreatedAPIKey,
// lib/api-keys.ts) doesn't even have that field, so there's no way this
// function could leak it into the list even by mistake.
export function toAPIKeyRow(key: APIKey, now: Date): APIKeyRow {
  return {
    id: key.id,
    keyPrefix: key.key_prefix,
    name: key.name,
    scopeLabels: key.scopes.map((s) => SCOPE_LABELS[s] ?? s).join(", "),
    isRevoked: key.revoked_at !== null,
    statusLabel: key.revoked_at !== null ? "Dicabut" : "Aktif",
    // "Belum pernah dipakai" is a real, distinct state from "used a long
    // time ago" — a key that's never sent a single request is exactly
    // what an Owner needs to notice ("integrasi mana yang tinggal
    // nama", PRD Phase 4 kebutuhan #2).
    lastUsedLabel: key.last_used_at ? formatApproximateID(key.last_used_at, now) : "Belum pernah dipakai",
  };
}
