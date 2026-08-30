import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/di/injection_container.dart';
import '../../../../shared/labels.dart';
import '../../../../shared/theme.dart';
import '../../../../shared/widgets/cache_banner.dart';
import '../bloc/lead_detail_bloc.dart';
import '../bloc/lead_detail_event.dart';
import '../bloc/leads_bloc.dart';
import '../bloc/leads_event.dart';
import '../bloc/leads_state.dart';
import '../widgets/lead_list_item.dart';
import 'lead_detail_page.dart';

/// Push, providing a fresh `LeadDetailBloc` for this one visit and
/// dispatching its initial load — the same `BlocProvider`-at-the-push-
/// site pattern the rest of this app doesn't have another Navigator
/// push to compare against yet, but mirrors how `AppShell` provides
/// `LeadsBloc` at its own creation site rather than inside `LeadsPage`.
void _openLeadDetail(BuildContext context, String leadId) {
  Navigator.of(context).push(
    MaterialPageRoute<void>(
      builder: (_) => BlocProvider<LeadDetailBloc>(
        create: (_) => sl<LeadDetailBloc>()..add(LeadDetailRequested(leadId)),
        child: const LeadDetailPage(),
      ),
    ),
  );
}

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
              Expanded(child: _Body(state: state)),
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
          hintText: 'Cari nama lead...',
          hintStyle: AppTextStyles.body.copyWith(
            color: AppColors.mutedForeground,
          ),
          prefixIcon: const Icon(
            Icons.search,
            color: AppColors.mutedForeground,
            size: 20,
          ),
          isDense: true,
          filled: true,
          fillColor: AppColors.surfaceSunken,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(AppRadius.pill),
            borderSide: BorderSide.none,
          ),
          contentPadding: const EdgeInsets.symmetric(vertical: 12),
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
      height: 48,
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
              ),
            ),
        ],
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  final String label;
  final bool isSelected;
  final VoidCallback onTap;

  const _StatusChip({
    required this.label,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppRadius.pill),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14),
        height: 32,
        decoration: BoxDecoration(
          color: isSelected ? AppColors.primary : AppColors.surfaceSunken,
          borderRadius: BorderRadius.circular(AppRadius.pill),
        ),
        alignment: Alignment.center,
        child: Text(
          label,
          style: TextStyle(
            fontSize: 12.5,
            fontWeight: FontWeight.w600,
            color: isSelected ? Colors.white : AppColors.mutedForeground,
          ),
        ),
      ),
    );
  }
}

class _Body extends StatelessWidget {
  final LeadsState state;

  const _Body({required this.state});

  @override
  Widget build(BuildContext context) {
    return switch (state) {
      LeadsInitial() || LeadsLoading() => const _LoadingSkeleton(),
      LeadsError(:final message) => _ErrorView(message: message),
      LeadsLoaded(:final leads) when leads.isEmpty => const _EmptyView(),
      LeadsLoaded(:final leads) => ListView.builder(
        physics: const AlwaysScrollableScrollPhysics(),
        itemCount: leads.length,
        itemBuilder: (context, index) => LeadListItem(
          lead: leads[index],
          onTap: () => _openLeadDetail(context, leads[index].id),
        ),
      ),
    };
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
