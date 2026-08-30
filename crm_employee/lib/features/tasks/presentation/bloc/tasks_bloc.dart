import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/failures.dart';
import '../../../auth/presentation/bloc/auth_bloc.dart';
import '../../../auth/presentation/bloc/auth_event.dart';
import '../../../auth/presentation/bloc/auth_state.dart';
import '../../domain/usecases/complete_task_usecase.dart';
import '../../domain/usecases/get_my_tasks_usecase.dart';
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

  Future<void> _load(Emitter<TasksState> emit) async {
    final authState = authBloc.state;
    if (authState is! AuthAuthenticated) return;

    emit(const TasksLoading());

    final result = await getMyTasks(
      GetMyTasksParams(assignedTo: authState.user.membershipId, status: 'open'),
    );

    result.fold(
      (failure) {
        if (failure is SessionExpiredFailure) {
          authBloc.add(const AuthSessionInvalidated());
          return;
        }
        emit(TasksError(failure.message));
      },
      (list) => emit(
        _sorted(
          TasksLoaded(
            tasks: list.tasks,
            fromCache: list.fromCache,
            fetchedAt: list.fetchedAt,
          ),
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
        fromCache: current.fromCache,
        fetchedAt: current.fetchedAt,
        completingTaskId: event.id,
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
              fromCache: reloaded.fromCache,
              fetchedAt: reloaded.fetchedAt,
              errorMessage: failure.message,
            ),
          );
        }
      },
      (_) async => _load(emit),
    );
  }

  /// Client-side, ascending by due date (nulls last) — `crm_be` sorts
  /// `GET /v1/tasks` by `created_at DESC` (confirmed directly in
  /// `repository_postgres.go`, no `due_at` order option exists), which
  /// isn't what "Tugas Saya, dengan jatuh tempo" (design brief §7.4) is
  /// actually for: knowing what's due soonest. Cheap and safe on one
  /// unpaginated page (#71's own no-pagination precedent).
  TasksLoaded _sorted(TasksLoaded state) {
    final sorted = [...state.tasks]..sort((a, b) {
      if (a.dueAt == null && b.dueAt == null) return 0;
      if (a.dueAt == null) return 1;
      if (b.dueAt == null) return -1;
      return a.dueAt!.compareTo(b.dueAt!);
    });
    return TasksLoaded(
      tasks: sorted,
      fromCache: state.fromCache,
      fetchedAt: state.fetchedAt,
    );
  }
}
