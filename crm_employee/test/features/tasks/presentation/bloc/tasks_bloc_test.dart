import 'package:bloc_test/bloc_test.dart';
import 'package:crm_employee/core/error/failures.dart';
import 'package:crm_employee/core/usecases/usecase.dart';
import 'package:crm_employee/features/auth/domain/entities/auth_user.dart';
import 'package:crm_employee/features/auth/domain/usecases/authenticate_with_biometrics_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/check_biometric_availability_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/check_stored_session_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/get_current_user_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/login_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/logout_usecase.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_bloc.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_event.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_state.dart';
import 'package:crm_employee/features/tasks/domain/entities/task.dart';
import 'package:crm_employee/features/tasks/domain/usecases/complete_task_usecase.dart';
import 'package:crm_employee/features/tasks/domain/usecases/get_my_tasks_usecase.dart';
import 'package:crm_employee/features/tasks/presentation/bloc/tasks_bloc.dart';
import 'package:crm_employee/features/tasks/presentation/bloc/tasks_event.dart';
import 'package:crm_employee/features/tasks/presentation/bloc/tasks_state.dart';
import 'package:dartz/dartz.dart' hide Task;
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockGetMyTasksUseCase extends Mock implements GetMyTasksUseCase {}

class MockCompleteTaskUseCase extends Mock implements CompleteTaskUseCase {}

class MockCheckStoredSessionUseCase extends Mock
    implements CheckStoredSessionUseCase {}

class MockCheckBiometricAvailabilityUseCase extends Mock
    implements CheckBiometricAvailabilityUseCase {}

class MockAuthenticateWithBiometricsUseCase extends Mock
    implements AuthenticateWithBiometricsUseCase {}

class MockLoginUseCase extends Mock implements LoginUseCase {}

class MockLogoutUseCase extends Mock implements LogoutUseCase {}

class MockGetCurrentUserUseCase extends Mock
    implements GetCurrentUserUseCase {}

Task _task({String id = 't1', String status = 'open', DateTime? dueAt}) =>
    Task(id: id, leadId: 'l1', title: 'Follow up', dueAt: dueAt, status: status, version: 1);

const _me = AuthUser(
  userId: 'u1',
  email: 'e@e.com',
  fullName: 'Budi',
  organizationId: 'o1',
  organizationName: 'Jualin',
  membershipId: 'm1',
  role: 'employee',
);

void main() {
  late MockGetMyTasksUseCase getMyTasks;
  late MockCompleteTaskUseCase completeTask;
  late MockLoginUseCase login;
  late MockGetCurrentUserUseCase getCurrentUser;
  late AuthBloc authBloc;

  setUpAll(() {
    registerFallbackValue(const GetMyTasksParams(assignedTo: 'm1'));
    registerFallbackValue(const CompleteTaskParams(id: 't1', version: 1));
  });

  setUp(() {
    getMyTasks = MockGetMyTasksUseCase();
    completeTask = MockCompleteTaskUseCase();
    login = MockLoginUseCase();
    getCurrentUser = MockGetCurrentUserUseCase();
    authBloc = AuthBloc(
      checkStoredSession: MockCheckStoredSessionUseCase(),
      checkBiometricAvailability: MockCheckBiometricAvailabilityUseCase(),
      authenticateWithBiometrics: MockAuthenticateWithBiometricsUseCase(),
      login: login,
      logout: MockLogoutUseCase(),
      getCurrentUser: getCurrentUser,
    );
  });

  tearDown(() => authBloc.close());

  TasksBloc buildBloc() =>
      TasksBloc(getMyTasks: getMyTasks, completeTask: completeTask, authBloc: authBloc);

  /// Real `AuthBloc`'s only externally-reachable path to
  /// `AuthAuthenticated` (its state is otherwise only settable from
  /// inside its own event handlers, `emit` is protected) — a real
  /// login, backed by mocked use cases. Awaited so `TasksBloc`'s own
  /// events, dispatched right after, see the already-settled state.
  Future<void> authenticate() async {
    when(
      () => login(const LoginParams(email: 'a@b.com', password: 'x')),
    ).thenAnswer((_) async => const Right(null));
    when(
      () => getCurrentUser(const NoParams()),
    ).thenAnswer((_) async => const Right(_me));
    authBloc.add(const AuthLoginSubmitted(email: 'a@b.com', password: 'x'));
    await authBloc.stream.firstWhere((s) => s is AuthAuthenticated);
  }

  group('TasksRequested', () {
    blocTest<TasksBloc, TasksState>(
      'not authenticated yet — no-op, never crashes reading membership_id',
      build: buildBloc,
      act: (bloc) => bloc.add(const TasksRequested()),
      expect: () => <TasksState>[],
    );

    blocTest<TasksBloc, TasksState>(
      "loads open tasks for the caller's own membership_id, sorted by due date ascending",
      setUp: () async {
        await authenticate();
        when(() => getMyTasks(any())).thenAnswer(
          (_) async => Right(
            TaskListResult(
              tasks: [
                _task(id: 't-late', dueAt: DateTime(2026, 9, 10)),
                _task(id: 't-soon', dueAt: DateTime(2026, 9, 1)),
                _task(id: 't-none'),
              ],
              fromCache: false,
            ),
          ),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const TasksRequested()),
      expect: () => [
        const TasksLoading(),
        isA<TasksLoaded>().having(
          (s) => s.tasks.map((t) => t.id).toList(),
          'tasks in due-date order',
          ['t-soon', 't-late', 't-none'],
        ),
      ],
      verify: (_) {
        verify(
          () => getMyTasks(const GetMyTasksParams(assignedTo: 'm1', status: 'open')),
        ).called(1);
      },
    );

    blocTest<TasksBloc, TasksState>(
      'SessionExpiredFailure dispatches AuthSessionInvalidated instead of emitting TasksError',
      setUp: () async {
        await authenticate();
        when(() => getMyTasks(any())).thenAnswer(
          (_) async => const Left(SessionExpiredFailure('Sesi Anda berakhir.')),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const TasksRequested()),
      expect: () => [const TasksLoading()],
      verify: (_) async {
        await Future<void>.delayed(const Duration(milliseconds: 50));
        expect(authBloc.state, isA<AuthSessionExpired>());
      },
    );
  });

  group('TaskCompletionRequested', () {
    blocTest<TasksBloc, TasksState>(
      'a successful completion refetches the list — the completed task disappears (status=open filter)',
      seed: () => TasksLoaded(tasks: [_task()], fromCache: false),
      setUp: () async {
        await authenticate();
        when(() => completeTask(any())).thenAnswer(
          (_) async => Right(_task(status: 'done')),
        );
        when(
          () => getMyTasks(any()),
        ).thenAnswer((_) async => const Right(TaskListResult(tasks: [], fromCache: false)));
      },
      build: buildBloc,
      act: (bloc) => bloc.add(
        const TaskCompletionRequested(id: 't1', version: 1),
      ),
      expect: () => [
        isA<TasksLoaded>().having(
          (s) => s.completingTaskId,
          'completingTaskId',
          't1',
        ),
        const TasksLoading(),
        const TasksLoaded(tasks: [], fromCache: false),
      ],
    );

    blocTest<TasksBloc, TasksState>(
      'a 409 refetches AND surfaces an inline error message — never a modal, never silently overwritten',
      seed: () => TasksLoaded(tasks: [_task()], fromCache: false),
      setUp: () async {
        await authenticate();
        when(() => completeTask(any())).thenAnswer(
          (_) async => Left(
            VersionConflictFailure<Task>(
              'Data sudah diubah oleh orang lain. Muat ulang dan coba lagi.',
              _task(status: 'done'),
            ),
          ),
        );
        when(() => getMyTasks(any())).thenAnswer(
          (_) async => Right(TaskListResult(tasks: [_task(status: 'done')], fromCache: false)),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(
        const TaskCompletionRequested(id: 't1', version: 1),
      ),
      expect: () => [
        isA<TasksLoaded>().having(
          (s) => s.completingTaskId,
          'completingTaskId',
          't1',
        ),
        const TasksLoading(),
        isA<TasksLoaded>().having((s) => s.errorMessage, 'errorMessage', isNull),
        isA<TasksLoaded>().having(
          (s) => s.errorMessage,
          'errorMessage',
          isNotNull,
        ),
      ],
    );
  });
}
