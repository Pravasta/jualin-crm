// Proves ActivityRepositoryImpl's orchestration of ActivityRemoteDataSource
// + ResponseCache — same shape as lead_repository_impl_test.dart's
// getMyLeads coverage, but exercising the bare-`List` envelope
// (Aturan #33's plain-list form) rather than `{data, meta}`.
import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/core/cache/response_cache.dart';
import 'package:crm_employee/features/leads/data/datasources/activity_remote_data_source.dart';
import 'package:crm_employee/features/leads/data/repositories/activity_repository_impl.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockActivityRemoteDataSource extends Mock
    implements ActivityRemoteDataSource {}

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

const _list = [
  {
    'id': 'a1',
    'lead_id': 'l1',
    'type': 'note_added',
    'actor_membership_id': 'm1',
    'body': 'Sudah dihubungi',
    'metadata': null,
    'created_at': '2026-08-30T00:00:00Z',
  },
];

void main() {
  late MockActivityRemoteDataSource remoteDataSource;
  late _FakeResponseCache cache;
  late ActivityRepositoryImpl repository;

  setUp(() {
    remoteDataSource = MockActivityRemoteDataSource();
    cache = _FakeResponseCache();
    repository = ActivityRepositoryImpl(
      remoteDataSource: remoteDataSource,
      responseCache: cache,
    );
  });

  group('getActivities', () {
    test('a successful network call returns the timeline, not from cache', () async {
      when(
        () => remoteDataSource.listActivities('l1'),
      ).thenAnswer((_) async => _list);

      final result = await repository.getActivities('l1');

      result.fold((f) => fail('expected Right, got Left(${f.message})'), (
        list,
      ) {
        expect(list.activities, hasLength(1));
        expect(list.activities.first.body, 'Sudah dihubungi');
        expect(list.fromCache, isFalse);
      });
    });

    test('network failure with a cache hit returns the cached timeline marked fromCache', () async {
      when(
        () => remoteDataSource.listActivities('l1'),
      ).thenAnswer((_) async => _list);
      await repository.getActivities('l1');

      when(
        () => remoteDataSource.listActivities('l1'),
      ).thenThrow(Exception('no connectivity'));

      final result = await repository.getActivities('l1');

      result.fold((f) => fail('expected Right, got Left(${f.message})'), (
        list,
      ) {
        expect(list.fromCache, isTrue);
        expect(list.fetchedAt, isNotNull);
      });
    });

    test('a real ApiError is never masked behind stale cache', () async {
      when(
        () => remoteDataSource.listActivities('l1'),
      ).thenAnswer((_) async => _list);
      await repository.getActivities('l1');

      when(() => remoteDataSource.listActivities('l1')).thenThrow(
        const ApiError(status: 404, code: 'not_found', message: 'Lead tidak ditemukan.'),
      );

      final result = await repository.getActivities('l1');

      expect(result.isLeft(), isTrue);
      result.fold(
        (f) => expect(f.message, 'Lead tidak ditemukan.'),
        (_) => fail('expected Left — a real 404 must not read stale cache'),
      );
    });
  });

  group('createActivity', () {
    test('posts the type/body and returns the created activity, never cached (a write)', () async {
      when(
        () => remoteDataSource.createActivity(
          leadId: 'l1',
          type: 'note_added',
          body: 'Follow up besok',
        ),
      ).thenAnswer(
        (_) async => {
          'id': 'a2',
          'lead_id': 'l1',
          'type': 'note_added',
          'actor_membership_id': 'm1',
          'body': 'Follow up besok',
          'metadata': null,
          'created_at': '2026-08-30T01:00:00Z',
        },
      );

      final result = await repository.createActivity(
        leadId: 'l1',
        type: 'note_added',
        body: 'Follow up besok',
      );

      result.fold((f) => fail('expected Right, got Left(${f.message})'), (
        activity,
      ) {
        expect(activity.id, 'a2');
        expect(activity.type, 'note_added');
        expect(activity.body, 'Follow up besok');
      });
    });
  });
}
