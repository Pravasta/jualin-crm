import { describe, expect, it } from "vitest";
import { canChangeRole, canDeactivate, roleOptionsFor, type TeamActor, type TeamRow } from "./team-permissions";

const OWNER: TeamActor = { membershipId: "m-owner", role: "owner" };
const ADMIN: TeamActor = { membershipId: "m-admin", role: "admin" };
const MANAGER: TeamActor = { membershipId: "m-manager", role: "manager" };

const OTHER_OWNER: TeamRow = { membershipId: "m-other-owner", role: "owner" };
const AN_ADMIN: TeamRow = { membershipId: "m-x", role: "admin" };
const A_MANAGER: TeamRow = { membershipId: "m-y", role: "manager" };
const AN_EMPLOYEE: TeamRow = { membershipId: "m-z", role: "employee" };

describe("canChangeRole", () => {
  it("nobody can change their own role — rule 3, no exception even for Owner", () => {
    expect(canChangeRole(OWNER, { membershipId: OWNER.membershipId, role: "owner" })).toBe(false);
    expect(canChangeRole(ADMIN, { membershipId: ADMIN.membershipId, role: "admin" })).toBe(false);
  });

  it("Admin cannot change an Owner's role — rule 4", () => {
    expect(canChangeRole(ADMIN, OTHER_OWNER)).toBe(false);
  });

  it("Owner CAN change another Owner's role (co-owner) — the design's mockup got this wrong", () => {
    expect(canChangeRole(OWNER, OTHER_OWNER)).toBe(true);
  });

  it("Admin can change non-Owner roles freely", () => {
    expect(canChangeRole(ADMIN, A_MANAGER)).toBe(true);
    expect(canChangeRole(ADMIN, AN_EMPLOYEE)).toBe(true);
  });
});

describe("roleOptionsFor", () => {
  it("Owner may assign any of the 4 roles, including Owner (co-owner)", () => {
    expect(roleOptionsFor(OWNER)).toEqual(["owner", "admin", "manager", "employee"]);
  });

  it('Admin never sees "Owner" as an option — promoting to Owner is rejected server-side regardless of target', () => {
    const options = roleOptionsFor(ADMIN);
    expect(options).not.toContain("owner");
    expect(options).toEqual(["admin", "manager", "employee"]);
  });
});

describe("canDeactivate", () => {
  it("nobody can deactivate themselves from this screen", () => {
    expect(canDeactivate(OWNER, { membershipId: OWNER.membershipId, role: "owner" })).toBe(false);
  });

  it("Admin cannot deactivate an Owner", () => {
    expect(canDeactivate(ADMIN, OTHER_OWNER)).toBe(false);
  });

  it("Owner can deactivate another Owner", () => {
    expect(canDeactivate(OWNER, OTHER_OWNER)).toBe(true);
  });

  it("Manager (read-only role, ActionMembershipDeactivate not granted) still gets a same-actor/target answer — the coarse gate is enforced elsewhere, not here", () => {
    // This function only encodes the RELATIONSHIP rules; the coarse
    // authz.Require(ActionMembershipDeactivate) gate that excludes
    // Manager entirely is checked at the screen level, not here.
    expect(canDeactivate(MANAGER, AN_ADMIN)).toBe(true);
  });
});
