import 'package:equatable/equatable.dart';

import '../../domain/entities/task.dart';

sealed class TasksState extends Equatable {
  const TasksState();

  @override
  List<Object?> get props => [];
}

class TasksInitial extends TasksState {
  const TasksInitial();
}

class TasksLoading extends TasksState {
  const TasksLoading();
}

class TasksLoaded extends TasksState {
  final List<Task> tasks;
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
    required this.fromCache,
    this.fetchedAt,
    this.completingTaskId,
    this.errorMessage,
  });

  @override
  List<Object?> get props => [
    tasks,
    fromCache,
    fetchedAt,
    completingTaskId,
    errorMessage,
  ];
}

class TasksError extends TasksState {
  final String message;

  const TasksError(this.message);

  @override
  List<Object?> get props => [message];
}
