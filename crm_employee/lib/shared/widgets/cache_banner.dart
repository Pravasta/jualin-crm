import 'package:flutter/material.dart';

import '../relative_time.dart';
import '../theme.dart';

/// TD §7 / design brief §10's "Dari cache, tanpa sinyal" banner — Lead
/// Saya (#71) and Detail Lead (#72) both need the exact same treatment,
/// so this is the second real implementation Aturan #28 asks for before
/// extracting (Lead Saya's own `_CacheBanner` was the first, folded into
/// this).
class CacheBanner extends StatelessWidget {
  final DateTime? fetchedAt;

  const CacheBanner({super.key, required this.fetchedAt});

  @override
  Widget build(BuildContext context) {
    final label = fetchedAt != null
        ? 'Data dari cache · diperbarui ${absoluteTime(fetchedAt!)}'
        : 'Data dari cache';
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.space20,
        vertical: AppSpacing.space8,
      ),
      color: AppColors.warningTint,
      child: Row(
        children: [
          const Icon(Icons.wifi_off, size: 15, color: AppColors.warning),
          const SizedBox(width: AppSpacing.space8),
          Text(
            label,
            style: const TextStyle(
              fontSize: 12.5,
              fontWeight: FontWeight.w600,
              color: AppColors.warning,
            ),
          ),
        ],
      ),
    );
  }
}
