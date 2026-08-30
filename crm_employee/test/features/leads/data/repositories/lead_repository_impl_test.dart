// Proves LeadRepositoryImpl's orchestration of LeadRemoteDataSource +
// ResponseCache — the offline-cache acceptance criterion (#2: "daftar
// lead tetap terbaca saat mode pesawat") starts here, at the layer that
// actually decides whether a result came from the network or the cache.
import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/core/cache/response_cache.dart';
import 'package:crm_employee/features/leads/data/datasources/lead_remote_data_source.dart';
import 'package:crm_employee/features/leads/data/repositories/lead_repository_impl.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockLeadRemoteDataSource extends Mock implements LeadRemoteDataSource {}

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
      'id': 'l1',
      'lead_number': 1024,
      'name': 'Rina Wijaya',
      'email': null,
      'phone': null,
      'company': null,
      'notes': null,
      'status': 'new',
      'lost_reason': null,
      'source': 'manual',
      'assigned_to_membership_id': 'm1',
      'version': 1,
      'created_at': '2026-08-30T00:00:00Z',
      'updated_at': '2026-08-30T00:00:00Z',
    },
  ],
  'meta': {'page': 1, 'per_page': 20, 'total': 1},
};

void main() {
  late MockLeadRemoteDataSource remoteDataSource;
  late _FakeResponseCache cache;
  late LeadRepositoryImpl repository;

  setUp(() {
    remoteDataSource = MockLeadRemoteDataSource();
    cache = _FakeResponseCache();
    repository = LeadRepositoryImpl(
      remoteDataSource: remoteDataSource,
      responseCache: cache,
    );
  });

  test('a successful network call returns leads with total, not from cache', () async {
    when(
      () => remoteDataSource.listMyLeads(status: null, query: null),
    ).thenAnswer((_) async => _envelope);

    final result = await repository.getMyLeads();

    result.fold((f) => fail('expected Right, got Left(${f.message})'), (
      list,
    ) {
      expect(list.leads, hasLength(1));
      expect(list.leads.first.name, 'Rina Wijaya');
      expect(list.total, 1);
      expect(list.fromCache, isFalse);
      expect(list.fetchedAt, isNull);
    });
  });

  test('network failure with a cache hit returns the cached leads marked fromCache', () async {
    // Prime the cache the way a prior successful call would have.
    when(
      () => remoteDataSource.listMyLeads(status: null, query: null),
    ).thenAnswer((_) async => _envelope);
    await repository.getMyLeads();

    when(
      () => remoteDataSource.listMyLeads(status: null, query: null),
    ).thenThrow(Exception('no connectivity'));

    final result = await repository.getMyLeads();

    result.fold((f) => fail('expected Right, got Left(${f.message})'), (
      list,
    ) {
      expect(list.leads, hasLength(1));
      expect(list.fromCache, isTrue);
      expect(list.fetchedAt, isNotNull);
    });
  });

  test('network failure with nothing cached surfaces as a Failure, not a crash', () async {
    when(
      () => remoteDataSource.listMyLeads(status: null, query: null),
    ).thenThrow(Exception('no connectivity'));

    final result = await repository.getMyLeads();

    expect(result.isLeft(), isTrue);
  });

  test('different status filters cache under different keys', () async {
    when(
      () => remoteDataSource.listMyLeads(status: 'new', query: null),
    ).thenAnswer((_) async => _envelope);
    when(
      () => remoteDataSource.listMyLeads(status: 'won', query: null),
    ).thenAnswer(
      (_) async => const {
        'data': <dynamic>[],
        'meta': {'page': 1, 'per_page': 20, 'total': 0},
      },
    );

    await repository.getMyLeads(status: 'new');
    await repository.getMyLeads(status: 'won');

    // Network now unreachable for both — each must fall back to ITS OWN
    // cached result, not accidentally share one cache entry.
    when(
      () => remoteDataSource.listMyLeads(status: 'new', query: null),
    ).thenThrow(Exception('no connectivity'));
    when(
      () => remoteDataSource.listMyLeads(status: 'won', query: null),
    ).thenThrow(Exception('no connectivity'));

    final newResult = await repository.getMyLeads(status: 'new');
    final wonResult = await repository.getMyLeads(status: 'won');

    expect(newResult.fold((_) => null, (l) => l.total), 1);
    expect(wonResult.fold((_) => null, (l) => l.total), 0);
  });

  test('a real ApiError (not connectivity) is never masked behind stale cache', () async {
    when(
      () => remoteDataSource.listMyLeads(status: null, query: null),
    ).thenAnswer((_) async => _envelope);
    await repository.getMyLeads();

    when(
      () => remoteDataSource.listMyLeads(status: null, query: null),
    ).thenThrow(
      const ApiError(
        status: 500,
        code: 'internal_error',
        message: 'Terjadi kesalahan internal.',
      ),
    );

    final result = await repository.getMyLeads();

    expect(result.isLeft(), isTrue);
    result.fold(
      (f) => expect(f.message, 'Terjadi kesalahan internal.'),
      (_) => fail('expected Left — a real server error must not read stale cache'),
    );
  });
}
