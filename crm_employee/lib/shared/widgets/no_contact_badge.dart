import 'package:flutter/material.dart';

import '../theme.dart';

/// "Belum ada kontak" — the marker `freeze.md` promised for a lead nobody can
/// call or email (issue #143). Words, not colour alone, and neutral rather than
/// red: a lead without contact is a legitimate state (a form that never asked,
/// an integration that never sent one), not an error anyone caused.
///
/// `foreground` on `surfaceSunken`, not `mutedForeground`: the theme documents
/// that token as 4.74:1 and restricted to metadata >= 13px, and this is 11.5px.
class NoContactBadge extends StatelessWidget {
  const NoContactBadge({super.key});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: AppColors.surfaceSunken,
        border: Border.all(color: AppColors.border),
        borderRadius: BorderRadius.circular(AppRadius.pill),
      ),
      child: const Text(
        'Belum ada kontak',
        style: TextStyle(
          color: AppColors.foreground,
          fontSize: 11.5,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
