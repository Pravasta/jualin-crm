import 'package:flutter/material.dart';

/// Navigation metadata and the pure logic around it — split out of the
/// app shell widget so which title/icon belongs to which tab is testable
/// without rendering anything (pola `crm_dashboard/src/lib/nav.ts` #40:
/// that file's `isActive`/`pageTitle` had a real bug class — a naive
/// prefix match made every route match "/" — worth guarding against here
/// even though Dart's exhaustive `switch` over an enum already rules out
/// the most direct equivalent, a missing case, at compile time).
///
/// Detail Lead is deliberately NOT a destination here — the design spec
/// (§4, kerangka aplikasi) is explicit that it's reached BY PUSH from
/// Lead Saya or Notifikasi, not a bottom-nav tab of its own.
enum AppDestination { leadSaya, tugasSaya, notifikasi }

String navTitle(AppDestination destination) => switch (destination) {
  AppDestination.leadSaya => 'Lead Saya',
  AppDestination.tugasSaya => 'Tugas Saya',
  AppDestination.notifikasi => 'Notifikasi',
};

IconData navIcon(AppDestination destination) => switch (destination) {
  AppDestination.leadSaya => Icons.home_outlined,
  AppDestination.tugasSaya => Icons.checklist_outlined,
  AppDestination.notifikasi => Icons.notifications_outlined,
};

/// Header avatar initials (design brief §4 — the header's only account
/// entry point). Pola `crm_dashboard/src/lib/nav.ts`'s `initialsOf`,
/// copied line-for-line: same rules, same edge cases.
String initialsOf(String fullName) {
  final parts = fullName
      .trim()
      .split(RegExp(r'\s+'))
      .where((p) => p.isNotEmpty)
      .toList();
  if (parts.isEmpty) return '?';
  if (parts.length == 1) {
    return parts[0].substring(0, parts[0].length.clamp(0, 2)).toUpperCase();
  }
  return (parts.first[0] + parts.last[0]).toUpperCase();
}
