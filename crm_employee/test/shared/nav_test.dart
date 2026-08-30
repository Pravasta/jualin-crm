// Proves shared/nav.dart's pure functions without rendering a single
// widget — the discipline crm_dashboard/src/lib/nav.ts's tests already
// established (#40), after a naive prefix match there made every route
// match "/". Dart's exhaustive `switch` over AppDestination rules out
// the direct equivalent (a missing case) at compile time; these tests
// cover the thing the compiler can't check — the actual mapped values.
import 'package:crm_employee/shared/nav.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('navTitle', () {
    test('every destination has a distinct, non-empty Indonesian title', () {
      final titles = AppDestination.values.map(navTitle).toList();
      for (final title in titles) {
        expect(title, isNotEmpty);
      }
      expect(titles.toSet(), hasLength(AppDestination.values.length));
    });

    test('maps the exact labels the design spec uses', () {
      expect(navTitle(AppDestination.leadSaya), 'Lead Saya');
      expect(navTitle(AppDestination.tugasSaya), 'Tugas Saya');
      expect(navTitle(AppDestination.notifikasi), 'Notifikasi');
    });
  });

  group('navIcon', () {
    test('every destination has a distinct icon', () {
      final icons = AppDestination.values.map(navIcon).toList();
      expect(icons.toSet(), hasLength(AppDestination.values.length));
    });
  });

  group('initialsOf', () {
    test('two-word name uses first letter of each word', () {
      expect(initialsOf('Rina Dewi'), 'RD');
    });

    test('single-word name uses its first two letters', () {
      expect(initialsOf('Budi'), 'BU');
    });

    test('three-or-more-word name uses first and last only', () {
      expect(initialsOf('Agus Setyo Prasetyo'), 'AP');
    });

    test('extra whitespace does not break splitting', () {
      expect(initialsOf('  Rina   Dewi  '), 'RD');
    });

    test('empty name falls back to a placeholder, never crashes', () {
      expect(initialsOf(''), '?');
      expect(initialsOf('   '), '?');
    });

    test('always uppercase regardless of input case', () {
      expect(initialsOf('rina dewi'), 'RD');
    });
  });
}
