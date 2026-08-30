import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/labels.dart';
import '../../../../shared/relative_time.dart';
import '../../../../shared/theme.dart';
import '../../../../shared/widgets/cache_banner.dart';
import '../../../auth/presentation/bloc/auth_bloc.dart';
import '../../../auth/presentation/bloc/auth_state.dart';
import '../../domain/entities/activity.dart';
import '../../domain/entities/lead.dart';
import '../../domain/lead_status.dart';
import '../activity_text.dart';
import '../bloc/lead_detail_bloc.dart';
import '../bloc/lead_detail_event.dart';
import '../bloc/lead_detail_state.dart';

/// Design brief §7.3 — the layar terpenting kedua. Telepon/WhatsApp stay
/// reachable without scrolling (a fixed bottom bar, not inside the
/// scroll view); status picker only ever offers valid transitions
/// (`lead_status.dart`); the conflict dialog (§8.2) is the only way a
/// rejected write reaches the user.
///
/// Takes no constructor params — the id it needs was already given to
/// `LeadDetailBloc` via `LeadDetailRequested` at the push call site
/// (`LeadsPage`'s `LeadListItem.onTap`), the same way `LeadsPage` itself
/// takes none.
class LeadDetailPage extends StatelessWidget {
  const LeadDetailPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Detail Lead')),
      body: BlocConsumer<LeadDetailBloc, LeadDetailState>(
        // noteError is deliberately NOT here — design brief §10 asks for
        // "kesalahan validasi per field" on the note form specifically,
        // so it renders inline under that field (`_NoteForm`), not as a
        // toast that could be missed or that outlives the field it's
        // about.
        listenWhen: (previous, current) {
          if (current is! LeadDetailLoaded) return false;
          final prev = previous is LeadDetailLoaded ? previous : null;
          return (current.conflict != null && prev?.conflict == null) ||
              (current.statusError != null &&
                  current.statusError != prev?.statusError) ||
              (current.externalActionError != null &&
                  current.externalActionError != prev?.externalActionError);
        },
        listener: (context, state) {
          final loaded = state as LeadDetailLoaded;
          if (loaded.conflict != null) {
            _showConflictDialog(context, loaded.conflict!);
          } else if (loaded.statusError != null) {
            _showErrorSnackBar(context, loaded.statusError!);
          } else if (loaded.externalActionError != null) {
            _showErrorSnackBar(context, loaded.externalActionError!);
          }
        },
        builder: (context, state) {
          return switch (state) {
            LeadDetailInitial() || LeadDetailLoading() => const _LoadingSkeleton(),
            LeadDetailError(:final leadId, :final message) => _ErrorView(
              message: message,
              onRetry: () => context.read<LeadDetailBloc>().add(
                LeadDetailRequested(leadId),
              ),
            ),
            LeadDetailLoaded() => _LoadedBody(state: state),
          };
        },
      ),
    );
  }

  void _showErrorSnackBar(BuildContext context, String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(
        SnackBar(content: Text(message), backgroundColor: AppColors.danger),
      );
  }

  void _showConflictDialog(BuildContext context, Lead current) {
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.dialog),
          ),
          title: const Text('Data sudah diubah'),
          // Design brief §8.2 — jujur tanpa menyalahkan pengguna.
          content: Text(
            'Lead ini sudah diubah oleh orang lain — sekarang berstatus '
            '"${statusMeta[current.status]?.label ?? current.status}". '
            'Muat ulang untuk melihat keadaan terkini sebelum mencoba lagi.',
          ),
          actions: [
            FilledButton(
              onPressed: () {
                Navigator.of(dialogContext).pop();
                dialogContext.read<LeadDetailBloc>().add(
                  const LeadStatusConflictAcknowledged(),
                );
              },
              child: const Text('Muat ulang'),
            ),
          ],
        );
      },
    );
  }
}

class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    return const Center(child: CircularProgressIndicator(color: AppColors.primary));
  }
}

class _ErrorView extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const _ErrorView({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.space40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, color: AppColors.danger, size: 40),
            const SizedBox(height: AppSpacing.space16),
            Text(message, textAlign: TextAlign.center, style: AppTextStyles.body),
            const SizedBox(height: AppSpacing.space16),
            OutlinedButton(onPressed: onRetry, child: const Text('Coba lagi')),
          ],
        ),
      ),
    );
  }
}

class _LoadedBody extends StatelessWidget {
  final LeadDetailLoaded state;

  const _LoadedBody({required this.state});

  @override
  Widget build(BuildContext context) {
    final lead = state.lead;
    final myMembershipId = switch (context.watch<AuthBloc>().state) {
      AuthAuthenticated(:final user) => user.membershipId,
      _ => null,
    };

    return Column(
      children: [
        Expanded(
          child: RefreshIndicator(
            color: AppColors.primary,
            onRefresh: () async {
              context.read<LeadDetailBloc>().add(
                const LeadDetailRefreshRequested(),
              );
              await Future<void>.delayed(const Duration(milliseconds: 400));
            },
            child: ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              children: [
                if (state.fromCache) CacheBanner(fetchedAt: state.fetchedAt),
                _LeadHeader(lead: lead),
                _StatusSection(state: state),
                const Divider(height: 1, color: AppColors.border),
                _NoteForm(
                  isSubmitting: state.isSubmittingNote,
                  error: state.noteError,
                ),
                const Divider(height: 1, color: AppColors.border),
                _Timeline(
                  activities: state.activities,
                  myMembershipId: myMembershipId,
                ),
                // Breathing room above the fixed action bar.
                const SizedBox(height: AppSpacing.space24),
              ],
            ),
          ),
        ),
        _ActionBar(lead: lead, isBusy: state.isLaunchingExternalAction),
      ],
    );
  }
}

class _LeadHeader extends StatelessWidget {
  final Lead lead;

  const _LeadHeader({required this.lead});

  @override
  Widget build(BuildContext context) {
    final meta = statusMeta[lead.status];

    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.space20,
        AppSpacing.space20,
        AppSpacing.space20,
        AppSpacing.space16,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(lead.name, style: AppTextStyles.screenTitle),
              ),
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
          const SizedBox(height: AppSpacing.space4),
          Text(
            '#${lead.leadNumber} · ${sourceLabels[lead.source] ?? lead.source} · '
            'disentuh ${relativeTime(lead.updatedAt)}',
            style: AppTextStyles.metadata,
          ),
          if (lead.company != null) ...[
            const SizedBox(height: AppSpacing.space12),
            Text(lead.company!, style: AppTextStyles.body),
          ],
          if (lead.email != null) ...[
            const SizedBox(height: AppSpacing.space4),
            Text(
              lead.email!,
              style: AppTextStyles.body.copyWith(color: AppColors.mutedForeground),
            ),
          ],
          if (lead.notes != null && lead.notes!.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.space12),
            Text(lead.notes!, style: AppTextStyles.body),
          ],
          if (lead.status == 'lost' && lead.lostReason != null) ...[
            const SizedBox(height: AppSpacing.space8),
            Text(
              'Alasan kalah: ${lostReasonDisplayLabel(lead.lostReason) ?? lead.lostReason}',
              style: AppTextStyles.metadata.copyWith(color: AppColors.danger),
            ),
          ],
        ],
      ),
    );
  }
}

class _StatusSection extends StatelessWidget {
  final LeadDetailLoaded state;

  const _StatusSection({required this.state});

  @override
  Widget build(BuildContext context) {
    final options = statusTransitionOptions(state.lead.status);

    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.space20,
        0,
        AppSpacing.space20,
        AppSpacing.space16,
      ),
      child: Row(
        children: [
          Expanded(
            child: Text(
              options.isEmpty ? 'Status ini bersifat final.' : 'Ubah status lead',
              style: AppTextStyles.body.copyWith(color: AppColors.mutedForeground),
            ),
          ),
          if (options.isNotEmpty)
            OutlinedButton(
              onPressed: state.isUpdatingStatus
                  ? null
                  : () => _openStatusPicker(context, state, options),
              child: state.isUpdatingStatus
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Ubah status'),
            ),
        ],
      ),
    );
  }

  Future<void> _openStatusPicker(
    BuildContext context,
    LeadDetailLoaded state,
    List<StatusTransitionOption> options,
  ) async {
    final bloc = context.read<LeadDetailBloc>();

    final selected = await showModalBottomSheet<StatusTransitionOption>(
      context: context,
      backgroundColor: AppColors.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.dialog)),
      ),
      builder: (sheetContext) => _StatusOptionsSheet(options: options),
    );
    if (selected == null) return;

    if (selected.status == 'lost') {
      if (!context.mounted) return;
      final reason = await showModalBottomSheet<String>(
        context: context,
        backgroundColor: AppColors.surface,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.dialog)),
        ),
        builder: (sheetContext) => const _LostReasonSheet(),
      );
      // Backed out of the follow-up step — design brief §9.2: "Kalah"
      // wajib disertai alasan, so an incomplete pick submits nothing.
      if (reason == null) return;
      bloc.add(LeadStatusChangeRequested('lost', lostReason: reason));
      return;
    }

    bloc.add(LeadStatusChangeRequested(selected.status));
  }
}

class _StatusOptionsSheet extends StatelessWidget {
  final List<StatusTransitionOption> options;

  const _StatusOptionsSheet({required this.options});

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.space20,
              AppSpacing.space20,
              AppSpacing.space20,
              AppSpacing.space8,
            ),
            child: Text('Ubah status ke', style: AppTextStyles.cardTitle),
          ),
          for (final option in options)
            ListTile(
              title: Text(option.label),
              onTap: () => Navigator.of(context).pop(option),
            ),
          const SizedBox(height: AppSpacing.space8),
        ],
      ),
    );
  }
}

class _LostReasonSheet extends StatelessWidget {
  const _LostReasonSheet();

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.space20,
              AppSpacing.space20,
              AppSpacing.space20,
              AppSpacing.space8,
            ),
            child: Text('Kenapa lead ini kalah?', style: AppTextStyles.cardTitle),
          ),
          for (final reason in lostReasons)
            ListTile(
              title: Text(lostReasonLabels[reason] ?? reason),
              onTap: () => Navigator.of(context).pop(reason),
            ),
          const SizedBox(height: AppSpacing.space8),
        ],
      ),
    );
  }
}

class _NoteForm extends StatefulWidget {
  final bool isSubmitting;

  /// Design brief §10 — "kesalahan validasi per field": rendered inline
  /// under the field itself via `InputDecoration.errorText`, not a
  /// toast.
  final String? error;

  const _NoteForm({required this.isSubmitting, required this.error});

  @override
  State<_NoteForm> createState() => _NoteFormState();
}

class _NoteFormState extends State<_NoteForm> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return BlocListener<LeadDetailBloc, LeadDetailState>(
      listenWhen: (previous, current) {
        final prev = previous is LeadDetailLoaded ? previous : null;
        final curr = current is LeadDetailLoaded ? current : null;
        // Submitted successfully — was in flight, now isn't, no error.
        return prev?.isSubmittingNote == true &&
            curr?.isSubmittingNote == false &&
            curr?.noteError == null;
      },
      listener: (context, state) => _controller.clear(),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.space20,
          AppSpacing.space16,
          AppSpacing.space20,
          AppSpacing.space16,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Tambah catatan', style: AppTextStyles.cardTitle),
            const SizedBox(height: AppSpacing.space8),
            TextField(
              controller: _controller,
              minLines: 2,
              maxLines: 4,
              style: AppTextStyles.body,
              decoration: InputDecoration(
                hintText: 'Tulis catatan...',
                errorText: widget.error,
              ),
            ),
            const SizedBox(height: AppSpacing.space8),
            Align(
              alignment: Alignment.centerRight,
              child: FilledButton(
                onPressed: widget.isSubmitting
                    ? null
                    : () => context.read<LeadDetailBloc>().add(
                        LeadNoteSubmitted(_controller.text),
                      ),
                style: FilledButton.styleFrom(
                  minimumSize: const Size(120, 44),
                ),
                child: widget.isSubmitting
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : const Text('Simpan'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Timeline extends StatelessWidget {
  final List<Activity> activities;
  final String? myMembershipId;

  const _Timeline({required this.activities, required this.myMembershipId});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.space20,
        AppSpacing.space16,
        AppSpacing.space20,
        0,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('Riwayat', style: AppTextStyles.cardTitle),
          const SizedBox(height: AppSpacing.space12),
          if (activities.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: AppSpacing.space16),
              child: Text(
                'Belum ada aktivitas.',
                style: AppTextStyles.body.copyWith(color: AppColors.mutedForeground),
              ),
            )
          else
            for (final activity in activities)
              _TimelineEntryTile(
                entry: activityToTimelineEntry(
                  activity,
                  myMembershipId: myMembershipId,
                ),
                createdAt: activity.createdAt,
                // Design brief §9.3 — the two mobile-originated types are
                // visually distinguished from everything else, including
                // notes/status changes/etc. from the dashboard.
                isMobileOriginated:
                    activity.type == 'call_logged' ||
                    activity.type == 'whatsapp_opened',
              ),
        ],
      ),
    );
  }
}

class _TimelineEntryTile extends StatelessWidget {
  final TimelineEntry entry;
  final DateTime createdAt;
  final bool isMobileOriginated;

  const _TimelineEntryTile({
    required this.entry,
    required this.createdAt,
    required this.isMobileOriginated,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.space16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            isMobileOriginated ? Icons.phone_iphone : Icons.circle,
            size: isMobileOriginated ? 16 : 8,
            color: isMobileOriginated ? AppColors.accentStrong : AppColors.mutedForeground,
          ),
          const SizedBox(width: AppSpacing.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(entry.text, style: AppTextStyles.body),
                const SizedBox(height: 2),
                Text(
                  [
                    if (entry.authorName != null) entry.authorName!,
                    relativeTime(createdAt),
                  ].join(' · '),
                  style: AppTextStyles.metadata,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ActionBar extends StatelessWidget {
  final Lead lead;
  final bool isBusy;

  const _ActionBar({required this.lead, required this.isBusy});

  @override
  Widget build(BuildContext context) {
    final hasPhone = lead.phone != null && lead.phone!.isNotEmpty;
    final hasWhatsApp = lead.phoneE164 != null;

    return SafeArea(
      top: false,
      child: Container(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.space20,
          AppSpacing.space12,
          AppSpacing.space20,
          AppSpacing.space12,
        ),
        decoration: const BoxDecoration(
          color: AppColors.surface,
          border: Border(top: BorderSide(color: AppColors.border)),
        ),
        child: Row(
          children: [
            Expanded(
              child: FilledButton.icon(
                onPressed: hasPhone && !isBusy
                    ? () => context.read<LeadDetailBloc>().add(
                        const LeadCallRequested(),
                      )
                    : null,
                icon: const Icon(Icons.call),
                label: const Text('Telepon'),
              ),
            ),
            const SizedBox(width: AppSpacing.space12),
            Expanded(
              child: OutlinedButton.icon(
                onPressed: hasWhatsApp && !isBusy
                    ? () => context.read<LeadDetailBloc>().add(
                        const LeadWhatsAppRequested(),
                      )
                    : null,
                icon: const Icon(Icons.chat),
                label: const Text('WhatsApp'),
                style: OutlinedButton.styleFrom(
                  minimumSize: const Size.fromHeight(52),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
