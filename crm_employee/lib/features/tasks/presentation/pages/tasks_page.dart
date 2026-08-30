import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/theme.dart';
import '../../../../shared/widgets/cache_banner.dart';
import '../../../leads/presentation/open_lead_detail.dart';
import '../bloc/tasks_bloc.dart';
import '../bloc/tasks_event.dart';
import '../bloc/tasks_state.dart';
import '../widgets/task_list_item.dart';

/// Design brief §7.4 — task list with due dates, one-way completion.
/// Always `status=open` (see `TasksBloc._load`'s doc comment) — no
/// filter UI, unlike Lead Saya's status chips, since the design brief
/// gives this screen no such state to build.
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
              if (state is TasksLoaded && state.fromCache)
                CacheBanner(fetchedAt: state.fetchedAt),
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
      TasksLoaded(:final tasks) when tasks.isEmpty => const _EmptyView(),
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
            Icons.check_circle_outline,
            color: AppColors.mutedForeground,
            size: 28,
          ),
        ),
        const SizedBox(height: AppSpacing.space20),
        const Text(
          'Tidak ada tugas terbuka',
          textAlign: TextAlign.center,
          style: AppTextStyles.cardTitle,
        ),
        const SizedBox(height: AppSpacing.space8),
        Text(
          'Tugas yang dibuat untuk Anda akan muncul di sini.',
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
