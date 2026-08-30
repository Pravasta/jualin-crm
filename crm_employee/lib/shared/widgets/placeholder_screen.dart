import 'package:flutter/material.dart';

import '../theme.dart';

/// Temporary. The app shell (issue #70) ships all three nav destinations
/// at once, but the screens behind them land one issue at a time
/// (#71–#73) — without this, two of the three tabs would show a blank
/// screen for several PR cycles. Pola `crm_dashboard`'s
/// `PlaceholderScreen` (#40), which #35 deleted outright once every
/// destination it stood in for existed for real.
///
/// Every call site is deleted by the issue that builds its real screen —
/// naming the issue here is what keeps that from being forgotten.
class PlaceholderScreen extends StatelessWidget {
  final String title;
  final String description;
  final int issue;

  const PlaceholderScreen({
    super.key,
    required this.title,
    required this.description,
    required this.issue,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.space40,
          vertical: AppSpacing.space24,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              title,
              textAlign: TextAlign.center,
              style: AppTextStyles.cardTitle,
            ),
            const SizedBox(height: AppSpacing.space8),
            Text(
              description,
              textAlign: TextAlign.center,
              style: AppTextStyles.body.copyWith(
                color: AppColors.mutedForeground,
              ),
            ),
            const SizedBox(height: AppSpacing.space12),
            Text(
              'Layar ini dibangun di issue #$issue.',
              textAlign: TextAlign.center,
              style: AppTextStyles.metadata,
            ),
          ],
        ),
      ),
    );
  }
}
