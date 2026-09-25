// Whether a lead can actually be followed up. freeze.md accepts leads with no
// contact at all — refusing one at the ingest point would throw away a
// customer — and promises the UI shows them "sebagai tidak dapat ditindaklanjuti".
// Until #143 nothing did. These pure functions are what decides it.
//
// A contact is an email OR a phone that is not blank. Blank means empty or
// whitespace-only, not just null: crm_be trims an email and stores what is
// left, so "   " can come back as "" — a naive `lead.email != null` would
// call that lead contactable. Company and notes are free text nobody can call,
// so they do not count. A phone that will not normalise for WhatsApp still
// does: it can be dialled.

interface ContactFields {
  email?: string | null;
  phone?: string | null;
}

function isBlank(value: string | null | undefined): boolean {
  return value == null || value.trim() === "";
}

export function hasContact(fields: ContactFields): boolean {
  return !isBlank(fields.email) || !isBlank(fields.phone);
}

// The soft confirmation when a lead is created by hand with no contact. Asked
// ONCE: after "Tetap simpan" the answer is settled and the save must go
// through — a check that fired again would be a block, which is exactly what
// this is not. Only the manual dialog uses it; form and API leads are never
// asked anything (nobody is there to ask).
export function shouldConfirmNoContact(fields: ContactFields, alreadyConfirmed: boolean): boolean {
  return !alreadyConfirmed && !hasContact(fields);
}

// The single contact shown in the lead table's Kontak column (#161). Phone
// first: it is what the list is scanned for ("who do I call"), and an email
// that follows would be truncated anyway. Blank means blank here too — a
// whitespace-only field must not print as an empty-looking cell.
export function primaryContact(fields: ContactFields): string {
  if (!isBlank(fields.phone)) return fields.phone!.trim();
  if (!isBlank(fields.email)) return fields.email!.trim();
  return "—";
}
