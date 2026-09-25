import 'package:flutter/material.dart';

import '../labels.dart';
import '../theme.dart';

/// Lead status badge — icon + label in the status color on its tint
/// (token sheet "Opsi A", Phase 8.6 #172). The one widget every screen uses,
/// so a status looks the same in Lead Saya, the detail header and anywhere
/// after. Unknown statuses render nothing rather than a blank pill.
class StatusBadge extends StatelessWidget {
  final String status;

  const StatusBadge({super.key, required this.status});

  @override
  Widget build(BuildContext context) {
    final meta = statusMeta[status];
    if (meta == null) return const SizedBox.shrink();

    return Semantics(
      label: 'Status ${meta.label}',
      excludeSemantics: true,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: meta.tint,
          borderRadius: BorderRadius.circular(6),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(meta.icon, size: 15, color: meta.color),
            const SizedBox(width: AppSpacing.space4 + 1),
            Flexible(
              child: Text(
                meta.label,
                style: TextStyle(
                  color: meta.color,
                  fontSize: 13,
                  height: 18 / 13,
                  fontWeight: FontWeight.w700,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
