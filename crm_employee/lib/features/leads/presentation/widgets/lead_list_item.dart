import 'package:flutter/material.dart';

import '../../../../shared/relative_time.dart';
import '../../../../shared/theme.dart';
import '../../../../shared/widgets/no_contact_badge.dart';
import '../../../../shared/widgets/status_badge.dart';
import '../../domain/entities/lead.dart';
import '../../domain/lead_contact.dart';

/// Design brief §6 — name, `#<lead_number>` (small, for talking to the
/// Owner by number), when last touched, and the status badge. Phase 8.6
/// (#173): a card, as in the handoff — separate cards read faster than a
/// ruled list when the phone is held at arm's length.
class LeadListItem extends StatelessWidget {
  final Lead lead;
  final VoidCallback? onTap;

  const LeadListItem({super.key, required this.lead, this.onTap});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.space12 - 2),
      child: Material(
        // Full width whatever the parent — ListView gives it that, but the
        // card shouldn't depend on it.
        type: MaterialType.card,
        color: AppColors.surface,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.card),
          side: const BorderSide(color: AppColors.border),
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: Container(
            width: double.infinity,
            padding: const EdgeInsets.all(AppSpacing.space16 - 2),
            // Name gets the full width; the badge sits under it, beside the
            // number. A right-aligned badge looked fine for "Baru" but
            // "Tidak Memenuhi Syarat" squeezed a long name into four lines
            // at 360dp (#173, seen in a render).
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  lead.name,
                  style: AppTextStyles.cardTitle.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: AppSpacing.space8),
                Wrap(
                  spacing: AppSpacing.space8,
                  runSpacing: AppSpacing.space4,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    StatusBadge(status: lead.status),
                    Text(
                      '#${lead.leadNumber} · disentuh ${relativeTime(lead.updatedAt)}',
                      style: AppTextStyles.metadata.copyWith(fontSize: 14),
                    ),
                  ],
                ),
                if (!leadHasContact(email: lead.email, phone: lead.phone)) ...[
                  const SizedBox(height: AppSpacing.space8),
                  const NoContactBadge(),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}
