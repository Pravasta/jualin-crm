import { describe, expect, it } from "vitest";
import { canManageAPIKeys, toAPIKeyRow } from "./api-key-rows";
import type { APIKey } from "./api-keys";

describe("canManageAPIKeys", () => {
  it("allows Owner and Admin", () => {
    expect(canManageAPIKeys("owner")).toBe(true);
    expect(canManageAPIKeys("admin")).toBe(true);
  });

  it("denies Manager and Employee — no access at all, not read-only", () => {
    expect(canManageAPIKeys("manager")).toBe(false);
    expect(canManageAPIKeys("employee")).toBe(false);
  });
});

function makeKey(overrides: Partial<APIKey> = {}): APIKey {
  return {
    id: "k-1",
    key_prefix: "jln_live_a3f9",
    name: "Website utama",
    scopes: ["leads:write"],
    created_by_membership_id: "m-1",
    created_at: "2026-08-01T00:00:00Z",
    last_used_at: null,
    revoked_at: null,
    expires_at: null,
    ...overrides,
  };
}

const NOW = new Date("2026-08-28T12:00:00Z");

describe("toAPIKeyRow", () => {
  it("maps an active key", () => {
    const row = toAPIKeyRow(makeKey(), NOW);
    expect(row.isRevoked).toBe(false);
    expect(row.statusLabel).toBe("Aktif");
    expect(row.scopeLabels).toBe("Kirim lead");
    expect(row.keyPrefix).toBe("jln_live_a3f9");
  });

  it("maps a revoked key — still a row, not filtered out", () => {
    const row = toAPIKeyRow(makeKey({ revoked_at: "2026-08-20T00:00:00Z" }), NOW);
    expect(row.isRevoked).toBe(true);
    expect(row.statusLabel).toBe("Dicabut");
  });

  it('never-used key gets "Belum pernah dipakai", not a formatted null date', () => {
    const row = toAPIKeyRow(makeKey({ last_used_at: null }), NOW);
    expect(row.lastUsedLabel).toBe("Belum pernah dipakai");
  });

  it("a used key gets the approximate label, not an exact timestamp", () => {
    const row = toAPIKeyRow(makeKey({ last_used_at: "2026-08-28T11:55:00Z" }), NOW);
    expect(row.lastUsedLabel).toBe("sekitar 5 menit lalu");
  });
});
