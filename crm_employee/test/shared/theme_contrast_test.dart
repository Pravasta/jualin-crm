// Phase 8.6 (#172): contrast is COMPUTED from the colors in the code, on
// every run — not copied from the token sheet. The design's printed ratios
// have been wrong before (#40, #70, and the dashboard's tint claim in #159);
// this test is what stops the next color tweak from quietly dropping a pair.
import 'package:crm_employee/shared/labels.dart';
import 'package:crm_employee/shared/theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

double contrast(Color a, Color b) {
  final la = a.computeLuminance();
  final lb = b.computeLuminance();
  final hi = la > lb ? la : lb;
  final lo = la > lb ? lb : la;
  return (hi + 0.05) / (lo + 0.05);
}

void main() {
  group('status badges (Opsi A) reach 7:1 — read in the sun', () {
    for (final status in leadStatuses) {
      test(status, () {
        final meta = statusMeta[status]!;
        expect(
          contrast(meta.color, meta.tint),
          greaterThanOrEqualTo(7.0),
          reason: '$status text on its tint',
        );
      });
    }

    test(
      'the two gray statuses differ by icon, since color cannot tell them apart',
      () {
        expect(
          statusMeta['unqualified']!.icon,
          isNot(statusMeta['spam']!.icon),
        );
      },
    );

    test('every status has its own icon', () {
      final icons = leadStatuses.map((s) => statusMeta[s]!.icon).toSet();
      expect(icons, hasLength(leadStatuses.length));
    });
  });

  group('theme text pairs', () {
    test(
      'primary text and metadata reach 7:1 on white and on the sunken surface',
      () {
        expect(
          contrast(AppColors.foreground, AppColors.surface),
          greaterThanOrEqualTo(7.0),
        );
        expect(
          contrast(AppColors.mutedForeground, AppColors.surface),
          greaterThanOrEqualTo(7.0),
        );
        expect(
          contrast(AppColors.mutedForeground, AppColors.surfaceSunken),
          greaterThanOrEqualTo(7.0),
        );
      },
    );

    test('white on primary clears AA (a button label, 16px semibold)', () {
      expect(
        contrast(Colors.white, AppColors.primary),
        greaterThanOrEqualTo(4.5),
      );
    });

    test('accent text and semantic pairs clear AA on their backgrounds', () {
      expect(
        contrast(AppColors.accentStrong, AppColors.surface),
        greaterThanOrEqualTo(7.0),
      );
      expect(
        contrast(AppColors.danger, AppColors.dangerTint),
        greaterThanOrEqualTo(4.5),
      );
      expect(
        contrast(AppColors.warning, AppColors.warningTint),
        greaterThanOrEqualTo(4.5),
      );
      expect(
        contrast(AppColors.success, AppColors.successTint),
        greaterThanOrEqualTo(4.5),
      );
    });
  });
}
