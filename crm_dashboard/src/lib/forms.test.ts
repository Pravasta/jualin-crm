import { describe, expect, it } from "vitest";
import { FIELD_KEYS, firstFieldConfigError, type Fields } from "./forms";

function baseFields(): Fields {
  return Object.fromEntries(
    FIELD_KEYS.map((k) => [k, { enabled: true, required: false, label: "X" }]),
  ) as Fields;
}

describe("firstFieldConfigError", () => {
  it("returns null when every field config is coherent", () => {
    expect(firstFieldConfigError(baseFields())).toBeNull();
  });

  it("rejects a field that is required but not enabled", () => {
    const fields = baseFields();
    fields.phone = { enabled: false, required: true, label: "Nomor" };
    expect(firstFieldConfigError(fields)).toEqual({
      key: "phone",
      message: "Field wajib harus diaktifkan lebih dulu.",
    });
  });

  it("rejects an enabled field with a blank label", () => {
    const fields = baseFields();
    fields.email = { enabled: true, required: false, label: "   " };
    expect(firstFieldConfigError(fields)).toEqual({
      key: "email",
      message: "Field yang aktif harus punya label.",
    });
  });

  it("does not flag a disabled field with a blank label", () => {
    const fields = baseFields();
    fields.company = { enabled: false, required: false, label: "" };
    expect(firstFieldConfigError(fields)).toBeNull();
  });

  it("checks fields in the canonical order", () => {
    const fields = baseFields();
    fields.name = { enabled: true, required: false, label: "" };
    fields.message = { enabled: false, required: true, label: "" };
    // name comes before message in FIELD_KEYS — its blank-label problem
    // is reported first.
    expect(firstFieldConfigError(fields)?.key).toBe("name");
  });
});
