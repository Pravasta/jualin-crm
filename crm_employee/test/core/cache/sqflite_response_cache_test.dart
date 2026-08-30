// Proves the real SqfliteResponseCache — actual SQL, not a fake behind
// the ResponseCache interface. sqflite has no platform channel in
// `flutter test`'s host environment, so `sqflite_common_ffi` (the
// package's own officially documented testing story) stands in for the
// real Android/iOS SQLite plugin — sqflite's public API
// (openDatabase/getDatabasesPath) is unchanged, only the underlying
// factory differs, so this exercises the exact same code path
// production uses.
import 'package:crm_employee/core/cache/sqflite_response_cache.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';

void main() {
  setUpAll(() {
    sqfliteFfiInit();
    databaseFactory = databaseFactoryFfi;
  });

  late SqfliteResponseCache cache;

  // All tests share one on-disk file (fixed filename, same as
  // production) — cleared explicitly here rather than trusting file
  // deletion between runs, so each test starts from a genuinely empty
  // table regardless of what a previous test in this file left behind.
  setUp(() async {
    cache = SqfliteResponseCache();
    await cache.clear();
  });

  test('get returns null when nothing was ever cached for a key', () async {
    expect(await cache.get('GET /v1/leads'), isNull);
  });

  test('put then get round-trips the exact body', () async {
    await cache.put('GET /v1/leads', '{"data":[{"id":"1"}]}');

    final result = await cache.get('GET /v1/leads');

    expect(result, isNotNull);
    expect(result!.body, '{"data":[{"id":"1"}]}');
  });

  test('fetchedAt reflects when put() was called, not when get() is', () async {
    final before = DateTime.now();
    await cache.put('GET /v1/leads', '{}');
    final after = DateTime.now();

    final result = await cache.get('GET /v1/leads');

    expect(
      result!.fetchedAt.isAfter(before.subtract(const Duration(seconds: 1))),
      isTrue,
    );
    expect(
      result.fetchedAt.isBefore(after.add(const Duration(seconds: 1))),
      isTrue,
    );
  });

  test('put on an existing key replaces it, does not duplicate', () async {
    await cache.put('GET /v1/leads', '{"data":[]}');
    await cache.put('GET /v1/leads', '{"data":[{"id":"1"}]}');

    final result = await cache.get('GET /v1/leads');

    expect(result!.body, '{"data":[{"id":"1"}]}');
  });

  test('different keys are stored independently', () async {
    await cache.put('GET /v1/leads', 'leads-body');
    await cache.put('GET /v1/tasks', 'tasks-body');

    expect((await cache.get('GET /v1/leads'))!.body, 'leads-body');
    expect((await cache.get('GET /v1/tasks'))!.body, 'tasks-body');
  });

  test('clear removes every row — logout must not leave anything behind', () async {
    await cache.put('GET /v1/leads', 'leads-body');
    await cache.put('GET /v1/tasks', 'tasks-body');

    await cache.clear();

    expect(await cache.get('GET /v1/leads'), isNull);
    expect(await cache.get('GET /v1/tasks'), isNull);
  });
}
