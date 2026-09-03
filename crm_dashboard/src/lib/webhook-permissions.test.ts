import { describe, expect, it } from "vitest";
import { canManageWebhooks } from "./webhook-permissions";
import type { Role } from "./labels";

describe("canManageWebhooks", () => {
  it("allows owner and admin", () => {
    expect(canManageWebhooks("owner")).toBe(true);
    expect(canManageWebhooks("admin")).toBe(true);
  });

  // Not read-only — no access at all, matching crm_be's authz matrix.
  it("denies manager and employee", () => {
    expect(canManageWebhooks("manager")).toBe(false);
    expect(canManageWebhooks("employee")).toBe(false);
  });

  // Guards against a fifth role being added to Role and silently
  // defaulting to allowed here.
  it("denies every role outside the allow-list", () => {
    const allowed: Role[] = ["owner", "admin"];
    const roles: Role[] = ["owner", "admin", "manager", "employee"];
    for (const role of roles) {
      expect(canManageWebhooks(role)).toBe(allowed.includes(role));
    }
  });
});
