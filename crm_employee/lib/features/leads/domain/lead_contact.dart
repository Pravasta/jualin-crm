/// Whether a lead can actually be followed up. `freeze.md` accepts leads with
/// no contact at all — refusing one at the ingest point would throw away a
/// customer — and promises the UI shows them "sebagai tidak dapat
/// ditindaklanjuti". Until issue #143 nothing did.
///
/// A contact is an email OR a phone that is not blank. Blank means empty or
/// whitespace-only, not just null: `crm_be` trims an email and stores what is
/// left, so "   " can come back as "" — a naive `lead.email != null` would call
/// that lead contactable. Company and notes are free text nobody can call, so
/// they do not count. A phone that will not normalise for WhatsApp still does:
/// it can be dialled.
bool _isBlank(String? value) => value == null || value.trim().isEmpty;

bool hasPhoneNumber(String? phone) => !_isBlank(phone);

bool leadHasContact({String? email, String? phone}) =>
    !_isBlank(email) || hasPhoneNumber(phone);
