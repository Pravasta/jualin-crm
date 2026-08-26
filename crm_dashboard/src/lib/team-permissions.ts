// Pure logic for what the Team screen offers per row — split out so the
// four relationship rules (docs/architecture/authorization.md "Empat
// aturan yang harus ditulis eksplisit") are testable without rendering
// React. These are NOT authz.Require's coarse role gate; they depend on
// actor-vs-target, which is exactly why crm_be enforces them inside
// internal/membership.Usecase rather than the generic map.
//
// The design's own mockup used a much simpler (and wrong) rule —
// `canChangeRole = !isSelf && role !== 'owner'` — which hides changing
// ANY owner's role regardless of who's asking. Real rule 4 only
// forbids it when the ACTOR is Admin; an Owner changing another Owner's
// role (a co-owner) is allowed (docs/architecture/authorization.md's
// "Catatan Aturan #4"). Matched to the backend exactly, same approach
// #33 took for status transitions.
import type { Role } from "./labels";

export interface TeamActor {
  membershipId: string;
  role: Role;
}

export interface TeamRow {
  membershipId: string;
  role: Role;
}

// Rule 3: nobody changes their own role, no exception, not even Owner.
// Rule 4: Admin can't touch an Owner's role (can't demote, can't
// promote anyone else TO Owner either — see roleOptionsFor below).
export function canChangeRole(actor: TeamActor, row: TeamRow): boolean {
  if (row.membershipId === actor.membershipId) return false;
  if (actor.role === "admin" && row.role === "owner") return false;
  return true;
}

// Which roles the <select> may offer. Rule 4's second half: Admin
// promoting anyone to Owner is rejected server-side regardless of the
// target — so Admin never sees "Owner" as a choice at all, not even for
// a row that isn't already an Owner.
export function roleOptionsFor(actor: TeamActor): Role[] {
  if (actor.role === "owner") return ["owner", "admin", "manager", "employee"];
  return ["admin", "manager", "employee"];
}

// Deactivation is a coarser question than role change: nobody
// self-deactivates from this screen at all (self-service removal isn't
// this button's job, and the one case where it's EVER valid — a
// non-last Owner — is rare enough not to special-case here), and Admin
// still can't touch an Owner.
export function canDeactivate(actor: TeamActor, row: TeamRow): boolean {
  if (row.membershipId === actor.membershipId) return false;
  if (actor.role === "admin" && row.role === "owner") return false;
  return true;
}
