// Coarse role gate for the webhook management screen (#103), sitting
// beside canManageForms and canManageAPIKeys — split out so it's testable
// without rendering React, same as lib/nav.ts and lib/team-permissions.ts.
import type { Role } from "./labels";

// Owner/Admin only. Manager and Employee get NO access at all, not
// read-only — crm_be's authz.go grants ActionWebhookCreate/List/Read/
// Update/Delete to those two roles alone, the same matrix as
// ActionAPIKey* and ActionForm*.
//
// The reason is the same one that governs the other two, and it is worth
// restating because a webhook endpoint looks harmless next to an API key:
// it is a standing instruction to send this organization's lead data to
// an address someone chose. Whoever can add one can quietly route every
// new lead somewhere else.
//
// This gate must sit ABOVE the fetch, not merely hide a button — issue
// #103's AC is "nol panggilan /v1/webhook-endpoints" for the other two
// roles, exactly the shape forms-screen.tsx and api-keys-screen.tsx use.
export function canManageWebhooks(role: Role): boolean {
  return role === "owner" || role === "admin";
}
