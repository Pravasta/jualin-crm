// Proves cachedGet's decision logic against a fake ResponseCache — no
// SQLite needed here, only what the function itself decides given an
// interface. sqflite_response_cache_test.dart separately proves the real
// implementation of that interface.
import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/core/cache/cached_get.dart';
import 'package:crm_employee/core/cache/response_cache.dart';
import 'package:flutter_test/flutter_test.dart';

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

void main() {
  late _FakeResponseCache cache;

  setUp(() => cache = _FakeResponseCache());

  test('a successful fetch caches the body and returns it fresh', () async {
    final result = await cachedGet(
      cache: cache,
      key: 'GET /v1/leads',
      fetch: () async => {'data': 'fresh'},
    );

    expect(result.fromCache, isFalse);
    expect(result.fetchedAt, isNull);
    expect(result.data, {'data': 'fresh'});
    expect((await cache.get('GET /v1/leads'))!.body, '{"data":"fresh"}');
  });

  test('network failure with a cache hit returns the cached body, marked fromCache', () async {
    await cache.put('GET /v1/leads', '{"data":"stale"}');

    final result = await cachedGet(
      cache: cache,
      key: 'GET /v1/leads',
      fetch: () => throw Exception('no connectivity'),
    );

    expect(result.fromCache, isTrue);
    expect(result.fetchedAt, isNotNull);
    expect(result.data, {'data': 'stale'});
  });

  test('network failure with nothing cached rethrows, does not swallow it', () async {
    await expectLater(
      () => cachedGet(
        cache: cache,
        key: 'GET /v1/leads',
        fetch: () => throw Exception('no connectivity'),
      ),
      throwsException,
    );
  });

  test('ApiError is never masked behind stale cache — a real server response wins', () async {
    await cache.put('GET /v1/leads', '{"data":"stale"}');

    await expectLater(
      () => cachedGet(
        cache: cache,
        key: 'GET /v1/leads',
        fetch: () => throw const ApiError(
          status: 500,
          code: 'internal_error',
          message: 'Terjadi kesalahan internal.',
        ),
      ),
      throwsA(isA<ApiError>()),
    );
  });

  test('SessionExpiredException is never masked behind stale cache either', () async {
    await cache.put('GET /v1/leads', '{"data":"stale"}');

    await expectLater(
      () => cachedGet(
        cache: cache,
        key: 'GET /v1/leads',
        fetch: () => throw const SessionExpiredException(),
      ),
      throwsA(isA<SessionExpiredException>()),
    );
  });
}
