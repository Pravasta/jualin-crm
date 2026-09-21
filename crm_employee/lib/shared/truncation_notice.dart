import 'package:flutter/material.dart';

import 'theme.dart';

/// Lists on this app load one page of up to [kListPageSize] items (issue
/// #152). Until then they sent no `per_page` at all and silently showed
/// crm_be's default of 25 — an employee with more simply never saw the rest.
const int kListPageSize = 100;

/// What to tell the user when a list shows fewer items than exist, or null
/// when it shows them all. Always names WHICH ones are missing: crm_be orders
/// these lists by `created_at DESC`, so it is the oldest that are cut — for
/// tasks, often the ones already overdue.
///
/// Null too when [total] is not larger than [shown] (a stale cached total, a
/// race): saying nothing beats announcing "-3 lead terlama".
String? truncationNotice({
  required int shown,
  required int total,
  required String noun,
  String? hint,
}) {
  final hidden = total - shown;
  if (hidden <= 0) return null;
  final base = 'Menampilkan $shown dari $total $noun. $hidden $noun terlama tidak ditampilkan.';
  return hint == null ? base : '$base $hint';
}

/// The banner form of [truncationNotice] — renders nothing when there is
/// nothing to say, so callers can place it unconditionally.
class TruncationNotice extends StatelessWidget {
  final String? message;

  const TruncationNotice({super.key, required this.message});

  @override
  Widget build(BuildContext context) {
    final text = message;
    if (text == null) return const SizedBox.shrink();
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.space20,
        vertical: AppSpacing.space8,
      ),
      color: AppColors.warningTint,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.info_outline, size: 16, color: AppColors.warning),
          const SizedBox(width: AppSpacing.space8),
          Expanded(
            child: Text(
              text,
              style: AppTextStyles.metadata.copyWith(color: AppColors.warning),
            ),
          ),
        ],
      ),
    );
  }
}
