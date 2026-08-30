import 'package:flutter/material.dart';

import '../../../../shared/relative_time.dart';
import '../../../../shared/theme.dart';
import '../../domain/entities/task.dart';

/// Design brief §7.4: "Daftar task, dengan jatuh tempo. Menandai selesai
/// satu arah — tidak bisa dibuka kembali." — a checkbox that only ever
/// goes one direction, disabled while its own completion request is in
/// flight (never the whole list). Tapping the ROW (not the checkbox)
/// opens the lead this task belongs to — not in the design brief's own
/// text, but a small, low-cost addition in the same spirit as the
/// app's "follow-up dari HP" mission; documented in notes.md `## #73`,
/// not silently assumed.
class TaskListItem extends StatelessWidget {
  final Task task;
  final bool isCompleting;
  final VoidCallback onComplete;
  final VoidCallback onTap;

  const TaskListItem({
    super.key,
    required this.task,
    required this.isCompleting,
    required this.onComplete,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final isDone = task.status == 'done';
    final overdue =
        !isDone && task.dueAt != null && task.dueAt!.isBefore(DateTime.now());

    return InkWell(
      onTap: isDone ? null : onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.space20,
          vertical: AppSpacing.space16,
        ),
        decoration: const BoxDecoration(
          border: Border(bottom: BorderSide(color: AppColors.border)),
        ),
        child: Row(
          children: [
            _Checkbox(
              checked: isDone,
              isBusy: isCompleting,
              onTap: isDone || isCompleting ? null : onComplete,
            ),
            const SizedBox(width: AppSpacing.space12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    task.title,
                    style: isDone
                        ? AppTextStyles.cardTitle.copyWith(
                            color: AppColors.mutedForeground,
                            decoration: TextDecoration.lineThrough,
                          )
                        : AppTextStyles.cardTitle,
                  ),
                  if (task.dueAt != null) ...[
                    const SizedBox(height: 3),
                    Text(
                      'Jatuh tempo ${relativeTime(task.dueAt!)}',
                      style: AppTextStyles.metadata.copyWith(
                        color: overdue
                            ? AppColors.danger
                            : AppColors.mutedForeground,
                        fontWeight: overdue ? FontWeight.w700 : FontWeight.w400,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Checkbox extends StatelessWidget {
  final bool checked;
  final bool isBusy;
  final VoidCallback? onTap;

  const _Checkbox({required this.checked, required this.isBusy, this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: SizedBox(
        width: kMinTouchTarget,
        height: kMinTouchTarget,
        child: Center(
          child: isBusy
              ? const SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Container(
                  width: 24,
                  height: 24,
                  decoration: BoxDecoration(
                    color: checked ? AppColors.success : Colors.transparent,
                    border: Border.all(
                      color: checked
                          ? AppColors.success
                          : AppColors.mutedForeground,
                      width: 1.5,
                    ),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: checked
                      ? const Icon(Icons.check, size: 16, color: Colors.white)
                      : null,
                ),
        ),
      ),
    );
  }
}
