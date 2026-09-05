import { describe, expect, it } from "vitest";
import type { Role } from "./labels";
import { canChangePlan, canViewSubscription } from "./subscription-permissions";

const ROLES: Role[] = ["owner", "admin", "manager", "employee"];

describe("canViewSubscription", () => {
  it("allows Owner and Admin only", () => {
    expect(canViewSubscription("owner")).toBe(true);
    expect(canViewSubscription("admin")).toBe(true);
    expect(canViewSubscription("manager")).toBe(false);
    expect(canViewSubscription("employee")).toBe(false);
  });
});

describe("canChangePlan", () => {
  it("allows Owner only — the first action Admin does not mirror Owner on", () => {
    for (const role of ROLES) {
      expect(canChangePlan(role)).toBe(role === "owner");
    }
  });
});
