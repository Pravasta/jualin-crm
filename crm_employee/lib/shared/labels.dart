import 'package:flutter/material.dart';

/// Backend enum values → what the user actually reads. Copied from
/// `crm_dashboard/src/lib/labels.ts`'s values (TD §3 — "menyalin nilai,
/// bukan mengimpornya": no Dart↔TypeScript code-sharing mechanism
/// exists, and building one would be an abstraction for a problem that
/// doesn't exist, Aturan #27). `glossary.md` stays the one source of
/// truth for the terms themselves; `test/shared/labels_test.dart` locks
/// each list to the same length as the backend enum (Aturan #12: seluruh
/// teks antarmuka Bahasa Indonesia).
///
/// Deliberately does NOT include `SCOPE_LABELS` (API key scopes) —
/// `crm_dashboard`'s labels.ts has that map because the dashboard manages
/// API keys; `crm_employee` never touches the API key format at all
/// (Aturan #24), so porting that map here would be the one piece of
/// `jln_*`-adjacent vocabulary this app has no business knowing.

// --- Lead status -----------------------------------------------------

const List<String> leadStatuses = [
  'new',
  'contacted',
  'qualified',
  'proposal',
  'won',
  'lost',
  'unqualified',
  'spam',
];

/// How a status is drawn in this app — the token sheet's "Opsi A" (Phase
/// 8.6, #172): an icon plus the label, in [color] on its own [tint]. On a
/// one-handed phone in the sun the icon is read before the text, so shape
/// carries the status and color never does alone. (The dashboard uses
/// "Opsi B", outline + shape: in a 25-row table eight icons turn into
/// noise.) Hues match the dashboard's, so a status looks like itself on
/// both.
///
/// Every [color]/[tint] pair is 7.01–7.13:1 — recomputed, and re-checked by
/// test/shared/theme_contrast_test.dart.
@immutable
class StatusMeta {
  final String label;

  /// Text and icon color.
  final Color color;

  /// Badge background.
  final Color tint;

  final IconData icon;

  const StatusMeta({
    required this.label,
    required this.color,
    required this.tint,
    required this.icon,
  });
}

const Map<String, StatusMeta> statusMeta = {
  'new': StatusMeta(
    label: 'Baru',
    color: Color(0xFF004EB3),
    tint: Color(0xFFEDF8FF),
    icon: Icons.add_circle_outline,
  ),
  'contacted': StatusMeta(
    label: 'Dihubungi',
    color: Color(0xFF673BA2),
    tint: Color(0xFFF9F4FF),
    icon: Icons.phone_outlined,
  ),
  'qualified': StatusMeta(
    label: 'Memenuhi Syarat',
    color: Color(0xFF005E60),
    tint: Color(0xFFEBFAFA),
    icon: Icons.check,
  ),
  'proposal': StatusMeta(
    label: 'Penawaran',
    color: Color(0xFF834300),
    tint: Color(0xFFFFF5E9),
    icon: Icons.description_outlined,
  ),
  'won': StatusMeta(
    label: 'Menang',
    color: Color(0xFF006307),
    tint: Color(0xFFEFFBEF),
    icon: Icons.star_outline,
  ),
  'lost': StatusMeta(
    label: 'Kalah',
    color: Color(0xFFAB000E),
    tint: Color(0xFFFFF2EF),
    icon: Icons.close,
  ),
  // The two statuses excluded from conversion rate are both near-gray on
  // purpose — so their ICONS must differ: "minus" (not a fit) vs "ban"
  // (junk). Color can't tell them apart (ΔE 3 on the dashboard, #171).
  'unqualified': StatusMeta(
    label: 'Tidak Memenuhi Syarat',
    color: Color(0xFF545454),
    tint: Color(0xFFF7F7F7),
    icon: Icons.remove_circle_outline,
  ),
  'spam': StatusMeta(
    label: 'Spam',
    color: Color(0xFF654F4B),
    tint: Color(0xFFF9F6F5),
    icon: Icons.block,
  ),
};

// --- Lost reason -------------------------------------------------------

const List<String> lostReasons = [
  'price',
  'competitor',
  'timing',
  'no_response',
  'not_interested',
  'other',
];

const Map<String, String> lostReasonLabels = {
  'price': 'Harga',
  'competitor': 'Kompetitor',
  'timing': 'Waktu Tidak Tepat',
  'no_response': 'Tidak Merespons',
  'not_interested': 'Tidak Tertarik',
  'other': 'Lainnya',
};

// --- Lead source ---------------------------------------------------------

const List<String> leadSources = ['manual', 'api', 'form', 'webhook'];

// "Formulir", not "Form" — the one source with a natural Indonesian
// word. API and Webhook stay as-is; they're proper nouns to the
// integrator who sees them (same reasoning as labels.ts).
const Map<String, String> sourceLabels = {
  'manual': 'Manual',
  'api': 'API',
  'form': 'Formulir',
  'webhook': 'Webhook',
};

// --- Role ----------------------------------------------------------------

const List<String> roles = ['owner', 'admin', 'manager', 'employee'];

// Left in English on purpose — glossary.md fixes these exact terms for
// the role enum, and the product speaks about "Owner"/"Admin" that way
// throughout, same as labels.ts.
const Map<String, String> roleLabels = {
  'owner': 'Owner',
  'admin': 'Admin',
  'manager': 'Manager',
  'employee': 'Employee',
};
