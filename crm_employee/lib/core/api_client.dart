import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import 'api_error.dart';
import 'config.dart';
import 'secure_store.dart';

typedef JsonMap = Map<String, dynamic>;

/// The client every screen talks to the API through — the Dart mirror of
/// `crm_dashboard/src/lib/api-client.ts` (TD §5), including its
/// single-flight refresh: N concurrent 401s must trigger exactly ONE call
/// to `/v1/auth/refresh`, or refresh token rotation (`crm_be` #10) reads
/// the second call's already-rotated token as reuse and revokes the whole
/// session out from under the user.
///
/// What's different from the dashboard, and why:
/// - Tokens live in [TokenStorage] (secure storage), not an `HttpOnly`
///   cookie — mobile's `client: "mobile"` login puts them in the response
///   body (TD §4), so this client owns reading/writing them explicitly.
/// - `authorize: false` calls (login, refresh itself) never enter the
///   401-triggers-refresh branch below. The dashboard's `apiFetch` does
///   let login flow through that branch — a wrong password there also
///   returns 401, which would otherwise attempt a refresh with no valid
///   session, then surface "Sesi Anda berakhir" instead of "Email atau
///   password salah". `authorize: false` on login sidesteps that class of
///   bug here rather than reproducing it.
class ApiClient {
  final http.Client _http;
  final TokenStorage tokens;
  final String _baseUrl;

  ApiClient({http.Client? httpClient, required this.tokens, String? baseUrl})
    : _http = httpClient ?? http.Client(),
      _baseUrl = baseUrl ?? AppConfig.apiBaseUrl;

  static const _refreshPath = '/v1/auth/refresh';

  // Module-level for a real ApiClient instance — shared by every send()
  // call this client makes. While non-null, every concurrent 401 awaits
  // this SAME future instead of starting its own refresh; reset to null
  // once it settles so the NEXT 401 starts a fresh cycle rather than
  // reusing a stale result (identical shape to api-client.ts's
  // `refreshPromise`).
  Future<bool>? _refreshFuture;

  Future<http.Response> _rawFetch(
    String path, {
    required String method,
    JsonMap? body,
    required bool authorize,
  }) async {
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (authorize) {
      final accessToken = await tokens.readAccessToken();
      if (accessToken != null) {
        headers['Authorization'] = 'Bearer $accessToken';
      }
    }

    final uri = Uri.parse('$_baseUrl$path');
    final encodedBody = body != null ? jsonEncode(body) : null;

    switch (method) {
      case 'GET':
        return _http.get(uri, headers: headers);
      case 'POST':
        return _http.post(uri, headers: headers, body: encodedBody);
      case 'PATCH':
        return _http.patch(uri, headers: headers, body: encodedBody);
      case 'DELETE':
        return _http.delete(uri, headers: headers, body: encodedBody);
      default:
        throw ArgumentError('unsupported method: $method');
    }
  }

  // doRefresh is entered at most once per refresh cycle no matter how
  // many callers are waiting — see _refreshFuture above.
  Future<bool> _doRefresh() {
    _refreshFuture ??= _performRefresh().whenComplete(() {
      _refreshFuture = null;
    });
    return _refreshFuture!;
  }

  // Calls rawFetch directly (never _fetchWithAuth) — a 401 from refresh
  // itself must never re-enter the retry logic below.
  Future<bool> _performRefresh() async {
    final refreshToken = await tokens.readRefreshToken();
    if (refreshToken == null) return false;

    try {
      final res = await _rawFetch(
        _refreshPath,
        method: 'POST',
        authorize: false,
        body: {'refresh_token': refreshToken},
      );
      if (res.statusCode != 200) {
        await tokens.clear();
        return false;
      }
      final data = (jsonDecode(res.body) as JsonMap)['data'] as JsonMap;
      await tokens.saveTokens(
        accessToken: data['access_token'] as String,
        refreshToken: data['refresh_token'] as String,
      );
      return true;
    } catch (_) {
      await tokens.clear();
      return false;
    }
  }

  Future<http.Response> _fetchWithAuth(
    String path, {
    required String method,
    JsonMap? body,
    required bool authorize,
    bool isRetry = false,
  }) async {
    final res = await _rawFetch(
      path,
      method: method,
      body: body,
      authorize: authorize,
    );

    final isRefreshCall = path == _refreshPath;
    if (res.statusCode == 401 && authorize && !isRefreshCall && !isRetry) {
      final refreshed = await _doRefresh();
      if (refreshed) {
        return _fetchWithAuth(
          path,
          method: method,
          body: body,
          authorize: authorize,
          isRetry: true,
        );
      }
      throw const SessionExpiredException();
    }
    return res;
  }

  /// Sends a request and returns the decoded `data` field of the
  /// `{data, meta}` envelope (Aturan #33). Throws [ApiError] on a
  /// non-2xx response, [SessionExpiredException] when a 401 couldn't be
  /// recovered by refresh.
  ///
  /// [authorize] controls two things at once: whether an `Authorization`
  /// header is attached, and whether a 401 may trigger the refresh dance
  /// — pass `false` for endpoints that run before a session exists
  /// (login, refresh itself).
  Future<dynamic> send(
    String path, {
    String method = 'GET',
    JsonMap? body,
    bool authorize = true,
  }) async {
    final res = await _fetchWithAuth(
      path,
      method: method,
      body: body,
      authorize: authorize,
    );
    return _decode(res);
  }

  dynamic _decode(http.Response res) {
    final text = res.body;
    final json = text.isNotEmpty ? jsonDecode(text) : null;

    if (res.statusCode < 200 || res.statusCode >= 300) {
      final errorBody =
          (json is JsonMap ? json['error'] as JsonMap? : null) ??
          {'code': 'internal_error', 'message': 'Terjadi kesalahan internal.'};
      throw ApiError.fromBody(res.statusCode, errorBody);
    }
    if (json is! JsonMap) return null;
    return json['data'];
  }
}
