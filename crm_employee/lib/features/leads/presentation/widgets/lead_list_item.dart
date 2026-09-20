import 'package:flutter/material.dart';

import '../../../../shared/labels.dart';
import '../../../../shared/relative_time.dart';
import '../../../../shared/theme.dart';
import '../../../../shared/widgets/no_contact_badge.dart';
import '../../domain/entities/lead.dart';
import '../../domain/lead_contact.dart';

/// Design brief §6 — name, `#<lead_number>` (small, for talking to the
/// Owner by number), when last touched, and a solid-fill status badge.
class LeadListItem extends StatelessWidget {
  final Lead lead;
  final VoidCallback? onTap;

  const LeadListItem({super.key, required this.lead, this.onTap});

  @override
  Widget build(BuildContext context) {
    final meta = statusMeta[lead.status];

    return InkWell(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.space20,
          vertical: AppSpacing.space16,
        ),
        decoration: const BoxDecoration(
          border: Border(bottom: BorderSide(color: AppColors.border)),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(lead.name, style: AppTextStyles.cardTitle),
                  const SizedBox(height: 3),
                  Text(
                    '#${lead.leadNumber} · disentuh ${relativeTime(lead.updatedAt)}',
                    style: AppTextStyles.metadata,
                  ),
                  if (!leadHasContact(
                    email: lead.email,
                    phone: lead.phone,
                  )) ...[
                    const SizedBox(height: AppSpacing.space4),
                    const NoContactBadge(),
                  ],
                ],
              ),
            ),
            const SizedBox(width: AppSpacing.space12),
            if (meta != null)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10),
                height: 26,
                decoration: BoxDecoration(
                  color: meta.background,
                  borderRadius: BorderRadius.circular(AppRadius.pill),
                ),
                alignment: Alignment.center,
                child: Text(
                  meta.label,
                  style: TextStyle(
                    color: meta.foreground,
                    fontSize: 11.5,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
