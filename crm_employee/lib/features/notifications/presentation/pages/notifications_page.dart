import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/relative_time.dart';
import '../../../../shared/theme.dart';
import '../../../leads/presentation/open_lead_detail.dart';
import '../../domain/entities/notification.dart';
import '../bloc/notifications_bloc.dart';
import '../bloc/notifications_event.dart';
import '../bloc/notifications_state.dart';

/// Design brief §7.5 — "Daftar notifikasi (lead baru ditugaskan).
/// Menekannya membuka lead terkait." No offline cache (see
/// `NotificationListResult`'s doc comment), no "mark all read" — neither
/// the design brief nor issue #73's own scope asks for one.
class NotificationsPage extends StatefulWidget {
  const NotificationsPage({super.key});

  @override
  State<NotificationsPage> createState() => _NotificationsPageState();
}

class _NotificationsPageState extends State<NotificationsPage> {
  @override
  void initState() {
    super.initState();
    context.read<NotificationsBloc>().add(const NotificationsRequested());
  }

  @override
  Widget build(BuildContext context) {
    return BlocBuilder<NotificationsBloc, NotificationsState>(
      builder: (context, state) {
        return RefreshIndicator(
          color: AppColors.primary,
          onRefresh: () async {
            context.read<NotificationsBloc>().add(
              const NotificationsRefreshRequested(),
            );
            await Future<void>.delayed(const Duration(milliseconds: 400));
          },
          child: _Body(state: state),
        );
      },
    );
  }
}

class _Body extends StatelessWidget {
  final NotificationsState state;

  const _Body({required this.state});

  @override
  Widget build(BuildContext context) {
    return switch (state) {
      NotificationsInitial() || NotificationsLoading() => const _LoadingSkeleton(),
      NotificationsError(:final message) => _ErrorView(message: message),
      NotificationsLoaded(:final notifications) when notifications.isEmpty =>
        const _EmptyView(),
      NotificationsLoaded(:final notifications) => ListView.builder(
        physics: const AlwaysScrollableScrollPhysics(),
        itemCount: notifications.length,
        itemBuilder: (context, index) {
          final item = notifications[index];
          return _NotificationTile(
            item: item,
            onTap: () {
              context.read<NotificationsBloc>().add(
                NotificationMarkReadRequested(item.id),
              );
              if (item.leadId != null) {
                openLeadDetail(context, item.leadId!);
              }
            },
          );
        },
      ),
    };
  }
}

class _NotificationTile extends StatelessWidget {
  final NotificationItem item;
  final VoidCallback onTap;

  const _NotificationTile({required this.item, required this.onTap});

  @override
  Widget build(BuildContext context) {
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
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Container(
                width: 8,
                height: 8,
                decoration: BoxDecoration(
                  color: item.isUnread ? AppColors.primary : Colors.transparent,
                  shape: BoxShape.circle,
                ),
              ),
            ),
            const SizedBox(width: AppSpacing.space12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    item.title,
                    style: item.isUnread
                        ? AppTextStyles.cardTitle
                        : AppTextStyles.cardTitle.copyWith(
                            color: AppColors.mutedForeground,
                          ),
                  ),
                  const SizedBox(height: 3),
                  Text(item.body, style: AppTextStyles.body),
                  const SizedBox(height: 3),
                  Text(
                    relativeTime(item.createdAt),
                    style: AppTextStyles.metadata,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.space20,
        vertical: AppSpacing.space8,
      ),
      itemCount: 4,
      itemBuilder: (context, index) => Container(
        height: 68,
        margin: const EdgeInsets.only(bottom: AppSpacing.space12),
        decoration: BoxDecoration(
          color: const Color(0xFFF0F0F0),
          borderRadius: BorderRadius.circular(AppRadius.card),
        ),
      ),
    );
  }
}

class _EmptyView extends StatelessWidget {
  const _EmptyView();

  @override
  Widget build(BuildContext context) {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.space40,
        vertical: AppSpacing.space24 * 4,
      ),
      children: [
        Container(
          width: 64,
          height: 64,
          decoration: const BoxDecoration(
            color: AppColors.surfaceSunken,
            shape: BoxShape.circle,
          ),
          child: const Icon(
            Icons.notifications_none,
            color: AppColors.mutedForeground,
            size: 28,
          ),
        ),
        const SizedBox(height: AppSpacing.space20),
        const Text(
          'Belum ada notifikasi',
          textAlign: TextAlign.center,
          style: AppTextStyles.cardTitle,
        ),
        const SizedBox(height: AppSpacing.space8),
        Text(
          'Lead baru yang ditugaskan ke Anda akan muncul di sini.',
          textAlign: TextAlign.center,
          style: AppTextStyles.body.copyWith(color: AppColors.mutedForeground),
        ),
      ],
    );
  }
}

class _ErrorView extends StatelessWidget {
  final String message;

  const _ErrorView({required this.message});

  @override
  Widget build(BuildContext context) {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.space40,
        vertical: AppSpacing.space24 * 4,
      ),
      children: [
        const Icon(Icons.error_outline, color: AppColors.danger, size: 40),
        const SizedBox(height: AppSpacing.space16),
        Text(message, textAlign: TextAlign.center, style: AppTextStyles.body),
      ],
    );
  }
}
