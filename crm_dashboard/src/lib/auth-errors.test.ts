import { describe, expect, it } from "vitest";
import { ApiError } from "./api-types";
import { globalMessage, isLeadAlreadyConverted, isLeadConvertedLocked } from "./auth-errors";

// Issue #142: crm_be answers 422 lead_converted_locked when a status change
// hits a lead that was already converted to a Customer. The screen hides the
// buttons in that case, so this is only reachable from a stale screen — the
// contract that matters is that it is recognised (to trigger a reload) and
// that its sentence reaches the banner untouched.
describe("isLeadConvertedLocked", () => {
  const message = "Lead ini sudah dikonversi menjadi Customer dan statusnya tidak dapat diubah lagi.";
  const locked = new ApiError(422, { code: "lead_converted_locked", message });

  it("recognises the code, and shows the backend's own sentence as-is", () => {
    expect(isLeadConvertedLocked(locked)).toBe(true);
    expect(globalMessage(locked)).toBe(message);
  });

  it("does not mistake its neighbours for it", () => {
    // invalid_status_transition is also a 422 about status; lead_already_converted
    // is the OTHER conversion error (a second POST /convert). Confusing either
    // with this one would reload the screen for the wrong reason.
    const invalid = new ApiError(422, { code: "invalid_status_transition", message: "x" });
    const twice = new ApiError(409, { code: "lead_already_converted", message: "x" });
    expect(isLeadConvertedLocked(invalid)).toBe(false);
    expect(isLeadConvertedLocked(twice)).toBe(false);
    expect(isLeadAlreadyConverted(locked)).toBe(false);
    expect(isLeadConvertedLocked(new Error("boom"))).toBe(false);
  });
});
