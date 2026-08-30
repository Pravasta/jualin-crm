// Mirrors crm_dashboard/src/lib/api-client.test.ts's two load-bearing
// cases (TD §12): refresh single-flight under real concurrency, and a
// failed refresh clearing the stored session. MockClient from
// package:http/testing.dart stands in for the network — no real crm_be
// needed for these two behaviors specifically (the manual end-to-end
// verification against a running crm_be covers the rest).
import 'dart:async';
import 'dart:convert';

import 'package:crm_employee/core/api_client.dart';
import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/core/secure_store.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

class _FakeTokenStorage implements TokenStorage {
  String? accessToken;
  String? refreshToken;
  int clearCalls = 0;
  int saveCalls = 0;

  _FakeTokenStorage({this.accessToken, this.refreshToken});

  @override
  Future<void> saveTokens({
    required String accessToken,
    required String refreshToken,
  }) async {
    saveCalls++;
    this.accessToken = accessToken;
    this.refreshToken = refreshToken;
  }

  @override
  Future<String?> readAccessToken() async => accessToken;

  @override
  Future<String?> readRefreshToken() async => refreshToken;

  @override
  Future<void> clear() async {
    clearCalls++;
    accessToken = null;
    refreshToken = null;
  }
}

http.Response _jsonResponse(int status, Map<String, dynamic> body) {
  return http.Response(jsonEncode(body), status);
}

void main() {
  group('ApiClient refresh single-flight', () {
    test(
      '6 concurrent 401s trigger exactly 1 call to /v1/auth/refresh',
      () async {
        final tokens = _FakeTokenStorage(
          accessToken: 'expired-access',
          refreshToken: 'valid-refresh',
        );
        var refreshCallCount = 0;
        final refreshGate = Completer<void>();

        final mockClient = MockClient((request) async {
          if (request.url.path == '/v1/auth/refresh') {
            refreshCallCount++;
            // Held open deliberately — every one of the 6 concurrent
            // calls below must have a chance to reach the single-flight
            // check while this is still pending, the same technique
            // api-client.test.ts's deferred() uses.
            await refreshGate.future;
            return _jsonResponse(200, {
              'data': {
                'access_token': 'new-access',
                'refresh_token': 'new-refresh',
                'access_token_ttl': 900,
                'refresh_token_ttl': 7776000,
              },
            });
          }

          final auth = request.headers['Authorization'];
          if (auth == 'Bearer new-access') {
            return _jsonResponse(200, {
              'data': {'ok': true},
            });
          }
          return _jsonResponse(401, {
            'error': {
              'code': 'authentication_required',
              'message': 'Sesi Anda berakhir.',
            },
          });
        });

        final apiClient = ApiClient(
          httpClient: mockClient,
          tokens: tokens,
          baseUrl: 'http://test.invalid',
        );

        const concurrentCalls = 6;
        final futures = List.generate(
          concurrentCalls,
          (_) => apiClient.send('/v1/protected'),
        );

        // Real wall-clock delay, not a microtask yield — gives every one
        // of the 6 calls above room to reach the 401-triggers-refresh
        // branch before refresh is allowed to complete.
        await Future<void>.delayed(const Duration(milliseconds: 20));
        refreshGate.complete();

        final results = await Future.wait(futures);

        expect(
          refreshCallCount,
          1,
          reason:
              'expected exactly 1 refresh call for $concurrentCalls concurrent 401s',
        );
        for (final result in results) {
          expect((result as Map)['ok'], isTrue);
        }
        expect(tokens.accessToken, 'new-access');
      },
    );

    test('a request that never 401s never triggers refresh', () async {
      final tokens = _FakeTokenStorage(
        accessToken: 'still-valid',
        refreshToken: 'valid-refresh',
      );
      var refreshCallCount = 0;

      final mockClient = MockClient((request) async {
        if (request.url.path == '/v1/auth/refresh') {
          refreshCallCount++;
        }
        return _jsonResponse(200, {
          'data': {'ok': true},
        });
      });

      final apiClient = ApiClient(
        httpClient: mockClient,
        tokens: tokens,
        baseUrl: 'http://test.invalid',
      );

      await apiClient.send('/v1/protected');

      expect(refreshCallCount, 0);
    });
  });

  group('ApiClient refresh failure', () {
    test(
      'a refresh that fails clears stored tokens and throws SessionExpiredException',
      () async {
        final tokens = _FakeTokenStorage(
          accessToken: 'expired-access',
          refreshToken: 'revoked-refresh',
        );

        final mockClient = MockClient((request) async {
          if (request.url.path == '/v1/auth/refresh') {
            return _jsonResponse(401, {
              'error': {
                'code': 'invalid_refresh_token',
                'message': 'Token tidak valid.',
              },
            });
          }
          return _jsonResponse(401, {
            'error': {
              'code': 'authentication_required',
              'message': 'Sesi Anda berakhir.',
            },
          });
        });

        final apiClient = ApiClient(
          httpClient: mockClient,
          tokens: tokens,
          baseUrl: 'http://test.invalid',
        );

        await expectLater(
          () => apiClient.send('/v1/protected'),
          throwsA(isA<SessionExpiredException>()),
        );
        expect(tokens.accessToken, isNull);
        expect(tokens.refreshToken, isNull);
        expect(tokens.clearCalls, greaterThanOrEqualTo(1));
      },
    );

    test(
      'no stored refresh token at all still ends in SessionExpiredException',
      () async {
        final tokens = _FakeTokenStorage(accessToken: 'expired-access');

        final mockClient = MockClient((request) async {
          return _jsonResponse(401, {
            'error': {
              'code': 'authentication_required',
              'message': 'Sesi Anda berakhir.',
            },
          });
        });

        final apiClient = ApiClient(
          httpClient: mockClient,
          tokens: tokens,
          baseUrl: 'http://test.invalid',
        );

        await expectLater(
          () => apiClient.send('/v1/protected'),
          throwsA(isA<SessionExpiredException>()),
        );
      },
    );
  });

  group('ApiClient authorize: false', () {
    test('a 401 on an unauthorized call never triggers refresh', () async {
      // Mirrors login's own behavior — a wrong password returning 401
      // must surface as ApiError(invalid_credentials), never attempt a
      // refresh with no session to refresh (see ApiClient's doc comment
      // on why this deliberately diverges from api-client.ts's literal
      // behavior).
      final tokens = _FakeTokenStorage(refreshToken: 'valid-refresh');
      var refreshCallCount = 0;

      final mockClient = MockClient((request) async {
        if (request.url.path == '/v1/auth/refresh') {
          refreshCallCount++;
        }
        return _jsonResponse(401, {
          'error': {
            'code': 'invalid_credentials',
            'message': 'Email atau password salah.',
          },
        });
      });

      final apiClient = ApiClient(
        httpClient: mockClient,
        tokens: tokens,
        baseUrl: 'http://test.invalid',
      );

      await expectLater(
        () => apiClient.send(
          '/v1/auth/login',
          method: 'POST',
          authorize: false,
          body: const {'email': 'a@b.com', 'password': 'wrong'},
        ),
        throwsA(
          isA<ApiError>().having((e) => e.code, 'code', 'invalid_credentials'),
        ),
      );
      expect(refreshCallCount, 0);
    });
  });
}
