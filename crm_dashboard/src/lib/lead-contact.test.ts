import { describe, expect, it } from "vitest";
import { hasContact, primaryContact, shouldConfirmNoContact } from "./lead-contact";

// Issue #143. "Punya kontak" is email OR phone that is not blank. The
// blank-handling is not pedantry: crm_be trims an email and stores what is
// left, so a submission of "   " can come back as "" rather than null — a
// lead that a naive `lead.email != null` would call contactable.

describe("hasContact", () => {
  it("is true when either one is present, or both", () => {
    expect(hasContact({ email: "budi@example.com", phone: null })).toBe(true);
    expect(hasContact({ email: null, phone: "0812 3456 7890" })).toBe(true);
    expect(hasContact({ email: "budi@example.com", phone: "0812" })).toBe(true);
  });

  it("is false when neither exists — null, undefined, or missing", () => {
    expect(hasContact({ email: null, phone: null })).toBe(false);
    expect(hasContact({})).toBe(false);
    expect(hasContact({ email: undefined, phone: undefined })).toBe(false);
  });

  it("treats an empty or whitespace-only value as absent", () => {
    expect(hasContact({ email: "", phone: "" })).toBe(false);
    expect(hasContact({ email: "   ", phone: "\t\n" })).toBe(false);
  });

  it("lets one real value carry a blank neighbour", () => {
    expect(hasContact({ email: "   ", phone: "0812" })).toBe(true);
    expect(hasContact({ email: "budi@example.com", phone: "  " })).toBe(true);
  });

  it("does not count company or notes — free text nobody can call", () => {
    const lead = { email: null, phone: null, company: "PT Maju Jaya", notes: "hubungi 0812 3456 7890" };
    expect(hasContact(lead)).toBe(false);
  });

  it("counts a phone that will not normalise for WhatsApp — it can still be dialled", () => {
    expect(hasContact({ email: null, phone: "ext 12" })).toBe(true);
  });
});

describe("shouldConfirmNoContact", () => {
  it("asks once when there is no contact and nobody has confirmed yet", () => {
    expect(shouldConfirmNoContact({ email: "", phone: "" }, false)).toBe(true);
    expect(shouldConfirmNoContact({ email: "  ", phone: null }, false)).toBe(true);
  });

  it("does not ask again once confirmed — 'Tetap simpan' must go through", () => {
    expect(shouldConfirmNoContact({ email: "", phone: "" }, true)).toBe(false);
  });

  it("never asks when there is a contact, confirmed or not", () => {
    expect(shouldConfirmNoContact({ email: "budi@example.com", phone: "" }, false)).toBe(false);
    expect(shouldConfirmNoContact({ email: "", phone: "0812" }, false)).toBe(false);
    expect(shouldConfirmNoContact({ email: "budi@example.com", phone: "" }, true)).toBe(false);
  });
});

describe("primaryContact", () => {
  it("prefers the phone, falls back to the email", () => {
    expect(primaryContact({ phone: "0812-3456-7890", email: "dewi@contoh.id" })).toBe("0812-3456-7890");
    expect(primaryContact({ phone: null, email: "sinar@contoh.id" })).toBe("sinar@contoh.id");
  });

  it("treats a whitespace-only phone as absent, same as hasContact", () => {
    expect(primaryContact({ phone: "   ", email: "a@contoh.id" })).toBe("a@contoh.id");
  });

  it("shows a dash when there is nothing to show", () => {
    expect(primaryContact({ phone: "", email: "  " })).toBe("—");
  });
});
