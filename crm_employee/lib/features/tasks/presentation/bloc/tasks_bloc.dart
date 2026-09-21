import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/failures.dart';
import '../../../auth/presentation/bloc/auth_bloc.dart';
import '../../../auth/presentation/bloc/auth_event.dart';
import '../../../auth/presentation/bloc/auth_state.dart';
import '../../domain/usecases/complete_task_usecase.dart';
import '../../domain/entities/task.dart';
import '../../domain/usecases/get_my_tasks_usecase.dart';
import 'task_filter.dart';
import 'tasks_event.dart';
import 'tasks_state.dart';

class TasksBloc extends Bloc<TasksEvent, TasksState> {
  final GetMyTasksUseCase getMyTasks;
  final CompleteTaskUseCase completeTask;

  /// Two jobs: dispatching `AuthSessionInvalidated` (same reasoning as
  /// `LeadsBloc.authBloc`), and reading the caller's own
  /// `membership_id` for `GetMyTasksParams.assignedTo` — read fresh from
  /// `authBloc.state` at request time, never cached at construction,
  /// since a stale value would silently show the wrong person's tasks
  /// after a session change.
  final AuthBloc authBloc;

  TasksBloc({
    required this.getMyTasks,
    required this.completeTask,
    required this.authBloc,
  }) : super(const TasksInitial()) {
    on<TasksRequested>(_onRequested);
    on<TasksRefreshRequested>(_onRefreshRequested);
    on<TaskCompletionRequested>(_onCompletionRequested);
    on<TaskFilterChanged>(_onFilterChanged);
  }

  Future<void> _onFilterChanged(
    TaskFilterChanged event,
    Emitter<TasksState> emit,
  ) async {
    await _load(emit, filter: event.filter);
  }

  Future<void> _onRequested(
    TasksRequested event,
    Emitter<TasksState> emit,
  ) async {
    await _load(emit);
  }

  Future<void> _onRefreshRequested(
    TasksRefreshRequested event,
    Emitter<TasksState> emit,
  ) async {
    await _load(emit);
  }

  /// [filter] defaults to whatever is showing now, so refresh and the
  /// reload after completing a task stay on the same tab.
  Future<void> _load(Emitter<TasksState> emit, {TaskFilter? filter}) async {
    final authState = authBloc.state;
    if (authState is! AuthAuthenticated) return;
    final active = filter ?? state.filter;

    emit(TasksLoading(filter: active));

    final result = await getMyTasks(
      GetMyTasksParams(
        assignedTo: authState.user.membershipId,
        status: active.status,
        perPage: active.perPage,
      ),
    );

    result.fold(
      (failure) {
        if (failure is SessionExpiredFailure) {
          authBloc.add(const AuthSessionInvalidated());
          return;
        }
        emit(TasksError(failure.message, filter: active));
      },
      (list) => emit(
        TasksLoaded(
          tasks: _sorted(list.tasks, active),
          total: list.total,
          fromCache: list.fromCache,
          fetchedAt: list.fetchedAt,
          filter: active,
        ),
      ),
    );
  }

  Future<void> _onCompletionRequested(
    TaskCompletionRequested event,
    Emitter<TasksState> emit,
  ) async {
    final current = state;
    if (current is! TasksLoaded) return;

    emit(
      TasksLoaded(
        tasks: current.tasks,
        total: current.total,
        fromCache: current.fromCache,
        fetchedAt: current.fetchedAt,
        completingTaskId: event.id,
        filter: current.filter,
      ),
    );

    final result = await completeTask(
      CompleteTaskParams(id: event.id, version: event.version),
    );

    await result.fold(
      (failure) async {
        if (failure is SessionExpiredFailure) {
          authBloc.add(const AuthSessionInvalidated());
          return;
        }
        // "Pesan inline + refetch" — same pattern #35's dashboard task
        // checkbox uses for its own 409 (never a modal like the lead
        // status conflict dialog, #72): a completed-elsewhere task is
        // cheap to just re-show as it actually stands now. Two emits
        // here (the plain reload, then this one layering the message on
        // top) rather than one combined — simpler than threading an
        // optional error param through `_load`'s shared success path,
        // and the extra frame is not something a person watching the
        // screen can perceive.
        await _load(emit);
        final reloaded = state;
        if (reloaded is TasksLoaded) {
          emit(
            TasksLoaded(
              tasks: reloaded.tasks,
              total: reloaded.total,
              fromCache: reloaded.fromCache,
              fetchedAt: reloaded.fetchedAt,
              errorMessage: failure.message,
              filter: reloaded.filter,
            ),
          );
        }
      },
      (_) async => _load(emit),
    );
  }

  /// Client-side — `crm_be` sorts `GET /v1/tasks` by `created_at DESC`
  /// (confirmed in `repository_postgres.go`; no other order option exists),
  /// which is what neither tab is for.
  ///
  /// Belum selesai: ascending by due date, nulls last — "Tugas Saya, dengan
  /// jatuh tempo" (design brief §7.4) is about what's due soonest.
  /// Selesai: most recently COMPLETED first — history reads backwards from
  /// now, and a task's due date says nothing about when it was done.
  List<Task> _sorted(List<Task> tasks, TaskFilter filter) {
    int nullsLast(DateTime? a, DateTime? b, int Function(DateTime, DateTime) cmp) {
      if (a == null && b == null) return 0;
      if (a == null) return 1;
      if (b == null) return -1;
      return cmp(a, b);
    }

    return [...tasks]..sort(
      (a, b) => filter == TaskFilter.done
          ? nullsLast(a.completedAt, b.completedAt, (x, y) => y.compareTo(x))
          : nullsLast(a.dueAt, b.dueAt, (x, y) => x.compareTo(y)),
    );
  }
}
