import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/labels.dart';
import '../../../../shared/theme.dart';
import '../../../../shared/widgets/cache_banner.dart';
import '../../../../shared/truncation_notice.dart';
import '../bloc/leads_bloc.dart';
import '../bloc/leads_event.dart';
import '../bloc/leads_state.dart';
import '../open_lead_detail.dart';
import '../widgets/lead_list_item.dart';

/// Design brief §6 — the layar terpenting. Status chip row + search +
/// list, offline cache banner (TD §7), and a pull-to-refresh that always
/// tries the network (never serves straight from cache on an explicit
/// gesture — that's what `LeadsRefreshRequested` means).
class LeadsPage extends StatefulWidget {
  const LeadsPage({super.key});

  @override
  State<LeadsPage> createState() => _LeadsPageState();
}

class _LeadsPageState extends State<LeadsPage> {
  final _searchController = TextEditingController();
  Timer? _debounce;

  @override
  void initState() {
    super.initState();
    context.read<LeadsBloc>().add(const LeadsRequested());
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged(String value) {
    // 300ms — the same debounce crm_dashboard's #32 used for its search
    // box, so a lead being typed doesn't trigger a request per keystroke.
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), () {
      if (!mounted) return;
      context.read<LeadsBloc>().add(LeadsSearchChanged(value));
    });
  }

  @override
  Widget build(BuildContext context) {
    return BlocBuilder<LeadsBloc, LeadsState>(
      builder: (context, state) {
        return RefreshIndicator(
          color: AppColors.primary,
          onRefresh: () async {
            context.read<LeadsBloc>().add(const LeadsRefreshRequested());
            // RefreshIndicator wants a Future — the bloc's own state
            // stream is what actually drives the UI, this just gives the
            // spinner something to await briefly.
            await Future<void>.delayed(const Duration(milliseconds: 400));
          },
          child: Column(
            children: [
              _SearchField(
                controller: _searchController,
                onChanged: _onSearchChanged,
              ),
              _StatusChipRow(
                selected: state.statusFilter,
                onSelected: (status) => context.read<LeadsBloc>().add(
                  LeadsStatusFilterChanged(status),
                ),
              ),
              if (state is LeadsLoaded && state.fromCache)
                CacheBanner(fetchedAt: state.fetchedAt),
              if (state is LeadsLoaded)
                TruncationNotice(
                  message: truncationNotice(
                    shown: state.leads.length,
                    total: state.total,
                    noun: 'lead',
                    hint: 'Persempit dengan status atau pencarian.',
                  ),
                ),
              Expanded(
                child: _Body(
                  state: state,
                  onClearFilters: () {
                    _debounce?.cancel();
                    _searchController.clear();
                    final bloc = context.read<LeadsBloc>();
                    bloc.add(const LeadsSearchChanged(''));
                    bloc.add(const LeadsStatusFilterChanged(null));
                  },
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _SearchField extends StatelessWidget {
  final TextEditingController controller;
  final ValueChanged<String> onChanged;

  const _SearchField({required this.controller, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.space20,
        AppSpacing.space12,
        AppSpacing.space20,
        AppSpacing.space8,
      ),
      child: TextField(
        controller: controller,
        onChanged: onChanged,
        style: AppTextStyles.body,
        decoration: InputDecoration(
          hintText: 'Cari nama lead',
          hintStyle: AppTextStyles.body.copyWith(
            color: AppColors.mutedForeground,
          ),
          prefixIcon: const Icon(
            Icons.search,
            color: AppColors.mutedForeground,
            size: 20,
          ),
          filled: true,
          fillColor: AppColors.surface,
          constraints: const BoxConstraints(minHeight: kMinTouchTarget),
          contentPadding: const EdgeInsets.symmetric(vertical: 14),
        ),
      ),
    );
  }
}

class _StatusChipRow extends StatelessWidget {
  final String? selected;
  final ValueChanged<String?> onSelected;

  const _StatusChipRow({required this.selected, required this.onSelected});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 52,
      child: ListView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.space20,
          vertical: AppSpacing.space8,
        ),
        children: [
          _StatusChip(
            label: 'Semua',
            isSelected: selected == null,
            onTap: () => onSelected(null),
          ),
          for (final status in leadStatuses)
            Padding(
              padding: const EdgeInsets.only(left: AppSpacing.space8),
              child: _StatusChip(
                label: statusMeta[status]!.label,
                isSelected: selected == status,
                onTap: () => onSelected(status),
                color: statusMeta[status]!.color,
              ),
            ),
        ],
      ),
    );
  }
}

/// A status filter chip — outlined in the status color, filled when
/// selected (the dashboard's chip, Phase 8.6 #173). Text is the status
/// color on white or white on the status color: both ≥7.5:1 with the
/// mobile scale (#172). "Semua" has no status, so it uses the foreground.
class _StatusChip extends StatelessWidget {
  final String label;
  final bool isSelected;
  final VoidCallback onTap;
  final Color color;

  const _StatusChip({
    required this.label,
    required this.isSelected,
    required this.onTap,
    this.color = AppColors.foreground,
  });

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: isSelected,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppRadius.pill),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14),
          height: 36,
          decoration: BoxDecoration(
            color: isSelected ? color : AppColors.surface,
            border: Border.all(color: color, width: 1.5),
            borderRadius: BorderRadius.circular(AppRadius.pill),
          ),
          alignment: Alignment.center,
          child: Text(
            label,
            style: TextStyle(
              fontSize: 13.5,
              fontWeight: FontWeight.w700,
              color: isSelected ? Colors.white : color,
            ),
          ),
        ),
      ),
    );
  }
}

class _Body extends StatelessWidget {
  final LeadsState state;
  final VoidCallback onClearFilters;

  const _Body({required this.state, required this.onClearFilters});

  @override
  Widget build(BuildContext context) {
    // Brief §13: "belum ada lead" and "tidak ada yang cocok" are different
    // answers — the first is not the Employee's to fix, the second is one
    // tap away (#173).
    final filtered = state.isFiltered;
    return switch (state) {
      LeadsInitial() || LeadsLoading() => const _LoadingSkeleton(),
      LeadsError(:final message) => _ErrorView(message: message),
      LeadsLoaded(:final leads) when leads.isEmpty && filtered =>
        _NoMatchView(onClearFilters: onClearFilters),
      LeadsLoaded(:final leads) when leads.isEmpty => const _EmptyView(),
      LeadsLoaded(:final leads) => ListView.builder(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.space16,
          AppSpacing.space4,
          AppSpacing.space16,
          AppSpacing.space16,
        ),
        itemCount: leads.length,
        itemBuilder: (context, index) => LeadListItem(
          lead: leads[index],
          onTap: () => openLeadDetail(context, leads[index].id),
        ),
      ),
    };
  }
}

class _NoMatchView extends StatelessWidget {
  final VoidCallback onClearFilters;

  const _NoMatchView({required this.onClearFilters});

  @override
  Widget build(BuildContext context) {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.space40,
        vertical: AppSpacing.space24 * 3,
      ),
      children: [
        const Text(
          'Tidak ada lead yang cocok',
          textAlign: TextAlign.center,
          style: AppTextStyles.cardTitle,
        ),
        const SizedBox(height: AppSpacing.space8),
        Text(
          'Tidak ada lead Anda dengan status atau nama ini.',
          textAlign: TextAlign.center,
          style: AppTextStyles.body.copyWith(color: AppColors.mutedForeground),
        ),
        const SizedBox(height: AppSpacing.space20),
        Center(
          child: OutlinedButton(
            onPressed: onClearFilters,
            style: OutlinedButton.styleFrom(
              minimumSize: const Size(0, kMinTouchTarget),
            ),
            child: const Text('Hapus filter'),
          ),
        ),
      ],
    );
  }
}

class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.space16,
        vertical: AppSpacing.space8,
      ),
      itemCount: 4,
      itemBuilder: (context, index) => Container(
        height: 84,
        margin: const EdgeInsets.only(bottom: AppSpacing.space12),
        decoration: BoxDecoration(
          color: AppColors.surfaceSunken,
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
            Icons.inbox_outlined,
            color: AppColors.mutedForeground,
            size: 28,
          ),
        ),
        const SizedBox(height: AppSpacing.space20),
        const Text(
          'Belum ada lead ditugaskan',
          textAlign: TextAlign.center,
          style: AppTextStyles.cardTitle,
        ),
        const SizedBox(height: AppSpacing.space8),
        Text(
          'Lead yang di-assign Owner ke Anda akan muncul di sini.',
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
        const Icon(
          Icons.error_outline,
          color: AppColors.danger,
          size: 40,
        ),
        const SizedBox(height: AppSpacing.space16),
        Text(
          message,
          textAlign: TextAlign.center,
          style: AppTextStyles.body,
        ),
      ],
    );
  }
}
