import { describe, expect, it } from "vitest";
import { canManageForms } from "./form-permissions";

describe("canManageForms", () => {
  it("allows Owner and Admin", () => {
    expect(canManageForms("owner")).toBe(true);
    expect(canManageForms("admin")).toBe(true);
  });

  it("denies Manager and Employee — same matrix as canManageAPIKeys", () => {
    expect(canManageForms("manager")).toBe(false);
    expect(canManageForms("employee")).toBe(false);
  });
});
