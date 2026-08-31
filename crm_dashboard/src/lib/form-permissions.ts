// Coarse role gate for the form management screen (#89), sitting next to
// canManageAPIKeys in api-key-rows.ts — split out so it's testable
// without rendering React, same as lib/nav.ts and lib/team-permissions.ts.
import type { Role } from "./labels";

// Owner/Admin only — Manager and Employee get NO access at all, not
// read-only (crm_be's authz.go: ActionFormCreate/List/Read/Update/Delete
// are all Owner/Admin, the same matrix as ActionAPIKey*). Like
// canManageAPIKeys there is no actor-vs-target relationship rule to
// layer on top: every Owner/Admin manages every form in the
// organization equally.
//
// This gate must sit ABOVE the fetch, not just hide a button — "Manager/
// Employee melihat pesan 'tidak tersedia untuk role Anda', nol panggilan
// API" (issue #89 AC), exactly the shape api-keys-screen.tsx uses.
export function canManageForms(role: Role): boolean {
  return role === "owner" || role === "admin";
}
