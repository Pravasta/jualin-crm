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

@immutable
class StatusMeta {
  final String label;

  /// Badge background — always solid-fill (design brief §11: "supaya
  /// kontras terjaga apa pun kondisi cahaya", the lesson Phase 3's #40
  /// text-on-tint badges didn't have to learn the hard way, but this
  /// design avoided from the start).
  final Color background;

  /// Badge text — always white on [background]. Every pair verified
  /// ≥4.5:1 independently (see `shared/theme.dart`'s doc comment and
  /// `notes.md`'s `## #70`), not copied from the design tool's own
  /// printed numbers.
  final Color foreground;

  const StatusMeta({
    required this.label,
    required this.background,
    required this.foreground,
  });
}

const Map<String, StatusMeta> statusMeta = {
  'new': StatusMeta(
    label: 'Baru',
    background: Color(0xFF0055A9),
    foreground: Colors.white,
  ),
  'contacted': StatusMeta(
    label: 'Dihubungi',
    background: Color(0xFF006B74),
    foreground: Colors.white,
  ),
  'qualified': StatusMeta(
    label: 'Memenuhi Syarat',
    background: Color(0xFF6529A9),
    foreground: Colors.white,
  ),
  'proposal': StatusMeta(
    // Gold, not a shade of the brand orange — design brief §11:
    // otherwise too easy to mistake for the primary action color, or
    // for "Kalah" (red), at a glance under bad light.
    label: 'Penawaran',
    background: Color(0xFF8D6000),
    foreground: Colors.white,
  ),
  'won': StatusMeta(
    label: 'Menang',
    background: Color(0xFF005F0E),
    foreground: Colors.white,
  ),
  'lost': StatusMeta(
    label: 'Kalah',
    background: Color(0xFFB00A1D),
    foreground: Colors.white,
  ),
  'unqualified': StatusMeta(
    label: 'Tidak Memenuhi Syarat',
    background: Color(0xFF474D5E),
    foreground: Colors.white,
  ),
  'spam': StatusMeta(
    label: 'Spam',
    background: Color(0xFF2E2E2E),
    foreground: Colors.white,
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
