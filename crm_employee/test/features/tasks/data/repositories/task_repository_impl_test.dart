// Proves TaskRepositoryImpl's orchestration of TaskRemoteDataSource +
// ResponseCache — same shape as lead_repository_impl_test.dart's
// getMyLeads coverage (TD §7 names /v1/tasks as one of the four
// cacheable endpoints).
import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/core/cache/response_cache.dart';
import 'package:crm_employee/core/error/failures.dart';
import 'package:crm_employee/features/tasks/data/datasources/task_remote_data_source.dart';
import 'package:crm_employee/features/tasks/data/repositories/task_repository_impl.dart';
import 'package:crm_employee/features/tasks/domain/entities/task.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockTaskRemoteDataSource extends Mock implements TaskRemoteDataSource {}

class _FakeResponseCache implements ResponseCache {
  final Map<String, CachedResponse> _store = {};

  @override
  Future<CachedResponse?> get(String key) async => _store[key];

  @override
  Future<void> put(String key, String body) async {
    _store[key] = CachedResponse(body: body, fetchedAt: DateTime.now());
  }

  @override
  Future<void> clear() async => _store.clear();
}

const _envelope = {
  'data': [
    {
      'id': 't1',
      'lead_id': 'l1',
      'title': 'Follow up telepon',
      'description': null,
      'due_at': '2026-09-01T00:00:00Z',
      'status': 'open',
      'version': 1,
    },
  ],
  'meta': {'page': 1, 'per_page': 20, 'total': 1},
};

void main() {
  late MockTaskRemoteDataSource remoteDataSource;
  late _FakeResponseCache cache;
  late TaskRepositoryImpl repository;

  setUp(() {
    remoteDataSource = MockTaskRemoteDataSource();
    cache = _FakeResponseCache();
    repository = TaskRepositoryImpl(
      remoteDataSource: remoteDataSource,
      responseCache: cache,
    );
  });

  group('getMyTasks', () {
    test('a successful network call returns tasks, not from cache', () async {
      when(
        () => remoteDataSource.listMyTasks(assignedTo: 'm1', status: 'open'),
      ).thenAnswer((_) async => _envelope);

      final result = await repository.getMyTasks(
        assignedTo: 'm1',
        status: 'open',
      );

      result.fold((f) => fail('expected Right, got Left(${f.message})'), (
        list,
      ) {
        expect(list.tasks, hasLength(1));
        expect(list.tasks.first.title, 'Follow up telepon');
        expect(list.fromCache, isFalse);
      });
    });

    test('network failure with a cache hit returns the cached tasks marked fromCache', () async {
      when(
        () => remoteDataSource.listMyTasks(assignedTo: 'm1', status: 'open'),
      ).thenAnswer((_) async => _envelope);
      await repository.getMyTasks(assignedTo: 'm1', status: 'open');

      when(
        () => remoteDataSource.listMyTasks(assignedTo: 'm1', status: 'open'),
      ).thenThrow(Exception('no connectivity'));

      final result = await repository.getMyTasks(
        assignedTo: 'm1',
        status: 'open',
      );

      result.fold((f) => fail('expected Right, got Left(${f.message})'), (
        list,
      ) {
        expect(list.fromCache, isTrue);
        expect(list.fetchedAt, isNotNull);
      });
    });

    test('a real ApiError is never masked behind stale cache', () async {
      when(
        () => remoteDataSource.listMyTasks(assignedTo: 'm1', status: 'open'),
      ).thenAnswer((_) async => _envelope);
      await repository.getMyTasks(assignedTo: 'm1', status: 'open');

      when(
        () => remoteDataSource.listMyTasks(assignedTo: 'm1', status: 'open'),
      ).thenThrow(
        const ApiError(status: 500, code: 'internal_error', message: 'Terjadi kesalahan internal.'),
      );

      final result = await repository.getMyTasks(
        assignedTo: 'm1',
        status: 'open',
      );

      expect(result.isLeft(), isTrue);
      result.fold(
        (f) => expect(f.message, 'Terjadi kesalahan internal.'),
        (_) => fail('expected Left — a real server error must not read stale cache'),
      );
    });
  });

  group('completeTask', () {
    const completedJson = {
      'id': 't1',
      'lead_id': 'l1',
      'title': 'Follow up telepon',
      'description': null,
      'due_at': '2026-09-01T00:00:00Z',
      'status': 'done',
      'version': 2,
    };

    test('a successful completion returns the task with the bumped version and status done', () async {
      when(
        () => remoteDataSource.complete(id: 't1', version: 1),
      ).thenAnswer((_) async => completedJson);

      final result = await repository.completeTask(id: 't1', version: 1);

      result.fold((f) => fail('expected Right, got Left(${f.message})'), (
        task,
      ) {
        expect(task.status, 'done');
        expect(task.version, 2);
      });
    });

    test('a 409 version_conflict surfaces as VersionConflictFailure<Task> carrying the server\'s current state', () async {
      when(() => remoteDataSource.complete(id: 't1', version: 1)).thenThrow(
        ApiError.fromBody(409, {
          'code': 'version_conflict',
          'message': 'Data sudah diubah oleh orang lain. Muat ulang dan coba lagi.',
          'current': completedJson,
        }),
      );

      final result = await repository.completeTask(id: 't1', version: 1);

      expect(result.isLeft(), isTrue);
      result.fold((failure) {
        expect(failure, isA<VersionConflictFailure<Task>>());
        final current = (failure as VersionConflictFailure<Task>).current;
        expect(current.status, 'done');
        expect(current.version, 2);
      }, (_) => fail('expected Left — a stale completion must never be silently accepted'));
    });
  });
}
