// Coarse role gate for the Langganan screen (#125), same shape as
// canManageWebhooks/canManageForms/canManageAPIKeys — split out so it's
// testable without rendering React, and so the screen's fetch sits
// behind it rather than merely hiding a button (AC: "nol panggilan API
// paket" for Manager/Employee).
import type { Role } from "./labels";

// crm_be's authz.ActionSubscriptionRead grants Owner AND Admin — billing
// is Owner's decision to make, but Admin (who runs day-to-day operations
// and hits plan_seat_limit_reached/plan_quota_exceeded from other
// screens) needs to see WHY, same reasoning TD 8.5 §9 gives.
export function canViewSubscription(role: Role): boolean {
  return role === "owner" || role === "admin";
}

// crm_be's authz.ActionSubscriptionChange (#124) is Owner ONLY — the
// first Action in the whole product where Admin does not mirror Owner.
// Billing is the one decision this product reserves for the Owner
// alone; Admin can see the test-checkout button's absence but never the
// button itself.
export function canChangePlan(role: Role): boolean {
  return role === "owner";
}
