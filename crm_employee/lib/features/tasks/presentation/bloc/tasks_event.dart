// dartz isn't imported here, so no `Task` collision — but equatable's
// own `Equatable` still applies.
import 'package:equatable/equatable.dart';

import 'task_filter.dart';

sealed class TasksEvent extends Equatable {
  const TasksEvent();

  @override
  List<Object?> get props => [];
}

/// Dispatched once when the tab is first shown.
class TasksRequested extends TasksEvent {
  const TasksRequested();
}

/// Pull-to-refresh — always tries the network first, same contract as
/// `LeadsRefreshRequested`.
class TasksRefreshRequested extends TasksEvent {
  const TasksRefreshRequested();
}

class TaskCompletionRequested extends TasksEvent {
  final String id;
  final int version;

  const TaskCompletionRequested({required this.id, required this.version});

  @override
  List<Object?> get props => [id, version];
}

/// Switching between Belum selesai and Selesai (issue #148).
class TaskFilterChanged extends TasksEvent {
  final TaskFilter filter;

  const TaskFilterChanged(this.filter);

  @override
  List<Object?> get props => [filter];
}
