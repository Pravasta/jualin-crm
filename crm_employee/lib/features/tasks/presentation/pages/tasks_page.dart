import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/theme.dart';
import '../../../../shared/truncation_notice.dart';
import '../../../../shared/widgets/cache_banner.dart';
import '../../../leads/presentation/open_lead_detail.dart';
import '../bloc/task_filter.dart';
import '../bloc/tasks_bloc.dart';
import '../bloc/tasks_event.dart';
import '../bloc/tasks_state.dart';
import '../widgets/task_list_item.dart';

/// Design brief §7.4 — task list with due dates, one-way completion.
/// Two tabs since issue #148: Belum selesai (the default, what it always
/// was) and Selesai, the history. Before that the screen only ever asked
/// for `status=open`, so a completed task simply vanished.
class TasksPage extends StatefulWidget {
  const TasksPage({super.key});

  @override
  State<TasksPage> createState() => _TasksPageState();
}

class _TasksPageState extends State<TasksPage> {
  @override
  void initState() {
    super.initState();
    context.read<TasksBloc>().add(const TasksRequested());
  }

  @override
  Widget build(BuildContext context) {
    return BlocConsumer<TasksBloc, TasksState>(
      listenWhen: (previous, current) {
        if (current is! TasksLoaded) return false;
        final prev = previous is TasksLoaded ? previous : null;
        return current.errorMessage != null &&
            current.errorMessage != prev?.errorMessage;
      },
      listener: (context, state) {
        final loaded = state as TasksLoaded;
        ScaffoldMessenger.of(context)
          ..hideCurrentSnackBar()
          ..showSnackBar(
            SnackBar(
              content: Text(loaded.errorMessage!),
              backgroundColor: AppColors.danger,
            ),
          );
      },
      builder: (context, state) {
        return RefreshIndicator(
          color: AppColors.primary,
          onRefresh: () async {
            context.read<TasksBloc>().add(const TasksRefreshRequested());
            await Future<void>.delayed(const Duration(milliseconds: 400));
          },
          child: Column(
            children: [
              _FilterBar(active: state.filter),
              if (state is TasksLoaded && state.fromCache)
                CacheBanner(fetchedAt: state.fetchedAt),
              if (state is TasksLoaded)
                TruncationNotice(
                  message: truncationNotice(
                    shown: state.tasks.length,
                    total: state.total,
                    noun: 'tugas',
                  ),
                ),
              Expanded(child: _Body(state: state)),
            ],
          ),
        );
      },
    );
  }
}

class _Body extends StatelessWidget {
  final TasksState state;

  const _Body({required this.state});

  @override
  Widget build(BuildContext context) {
    return switch (state) {
      TasksInitial() || TasksLoading() => const _LoadingSkeleton(),
      TasksError(:final message) => _ErrorView(message: message),
      TasksLoaded(:final tasks, :final filter) when tasks.isEmpty =>
        _EmptyView(filter: filter),
      TasksLoaded(:final tasks, :final completingTaskId) => ListView.builder(
        physics: const AlwaysScrollableScrollPhysics(),
        itemCount: tasks.length,
        itemBuilder: (context, index) {
          final task = tasks[index];
          return TaskListItem(
            task: task,
            isCompleting: completingTaskId == task.id,
            onComplete: () => context.read<TasksBloc>().add(
              TaskCompletionRequested(id: task.id, version: task.version),
            ),
            onTap: () => openLeadDetail(context, task.leadId),
          );
        },
      ),
    };
  }
}

class _FilterBar extends StatelessWidget {
  final TaskFilter active;

  const _FilterBar({required this.active});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.space20,
        AppSpacing.space12,
        AppSpacing.space20,
        AppSpacing.space8,
      ),
      child: SizedBox(
        width: double.infinity,
        child: SegmentedButton<TaskFilter>(
          showSelectedIcon: false,
          segments: [
            for (final f in TaskFilter.values)
              ButtonSegment(value: f, label: Text(f.label)),
          ],
          selected: {active},
          onSelectionChanged: (selection) {
            final next = selection.first;
            if (next != active) {
              context.read<TasksBloc>().add(TaskFilterChanged(next));
            }
          },
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
        height: 60,
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
  final TaskFilter filter;

  const _EmptyView({required this.filter});

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
            Icons.check_circle_outline,
            color: AppColors.mutedForeground,
            size: 28,
          ),
        ),
        const SizedBox(height: AppSpacing.space20),
        Text(
          filter == TaskFilter.done
              ? 'Belum ada tugas yang selesai'
              : 'Tidak ada tugas terbuka',
          textAlign: TextAlign.center,
          style: AppTextStyles.cardTitle,
        ),
        const SizedBox(height: AppSpacing.space8),
        Text(
          filter == TaskFilter.done
              ? 'Tugas yang Anda tandai selesai akan tercatat di sini.'
              : 'Tugas yang dibuat untuk Anda akan muncul di sini.',
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
