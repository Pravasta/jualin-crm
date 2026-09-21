import 'package:equatable/equatable.dart';

import '../../domain/entities/task.dart';
import 'task_filter.dart';

/// Every state carries the active [TaskFilter] (issue #148), so the tab
/// stays selected through loading, refresh, and errors — the screen never
/// has to remember it separately and drift from what the bloc loaded.
sealed class TasksState extends Equatable {
  final TaskFilter filter;

  const TasksState({this.filter = TaskFilter.open});

  @override
  List<Object?> get props => [filter];
}

class TasksInitial extends TasksState {
  const TasksInitial();
}

class TasksLoading extends TasksState {
  const TasksLoading({super.filter});
}

class TasksLoaded extends TasksState {
  final List<Task> tasks;

  /// `meta.total` from the server (issue #152) — so the screen can say when
  /// [tasks] is not everything. A getter so the constructor stays `const`.
  final int? _total;
  int get total => _total ?? tasks.length;
  final bool fromCache;
  final DateTime? fetchedAt;

  /// Non-null while that row's checkbox is mid-request — disables just
  /// that row, not the whole list.
  final String? completingTaskId;

  /// Aturan #35's 409, handled the SAME way #35's dashboard task
  /// checkbox handles it (`docs/phases/03-owner-dashboard/notes.md`):
  /// an inline message, not a modal like the lead status conflict dialog
  /// (#72) — completing a task is cheap to retry, unlike a lead status
  /// change that might need a lost-reason re-entered.
  final String? errorMessage;

  const TasksLoaded({
    required this.tasks,
    int? total,
    required this.fromCache,
    this.fetchedAt,
    this.completingTaskId,
    this.errorMessage,
    super.filter,
    // A named parameter cannot start with `_`, so `this._total` is not an
    // option here; the lint's suggestion does not compile.
    // ignore: prefer_initializing_formals
  }) : _total = total;

  @override
  List<Object?> get props => [
    tasks,
    total,
    fromCache,
    fetchedAt,
    completingTaskId,
    errorMessage,
    filter,
  ];
}

class TasksError extends TasksState {
  final String message;

  const TasksError(this.message, {super.filter});

  @override
  List<Object?> get props => [message, filter];
}
