import '../../core/api_client.dart';
import '../../core/secure_store.dart';

/// Typed wrappers for `/v1/auth/*` and `/v1/me` — shapes read directly from
/// `crm_be/internal/auth/{handler_http,entity}.go`, mirroring
/// `crm_dashboard/src/lib/auth.ts`. Always sends `client: "mobile"`
/// (`crm_be`'s `ClientMobile`) — the field that makes the handler respond
/// with tokens in the body instead of `Set-Cookie` (TD §4).
class MeResult {
  final String userId;
  final String email;
  final String fullName;
  final String organizationId;
  final String organizationName;
  final String membershipId;
  final String role;

  const MeResult({
    required this.userId,
    required this.email,
    required this.fullName,
    required this.organizationId,
    required this.organizationName,
    required this.membershipId,
    required this.role,
  });

  factory MeResult.fromJson(JsonMap json) {
    return MeResult(
      userId: json['user_id'] as String,
      email: json['email'] as String,
      fullName: json['full_name'] as String,
      organizationId: json['organization_id'] as String,
      organizationName: json['organization_name'] as String,
      membershipId: json['membership_id'] as String,
      role: json['role'] as String,
    );
  }
}

class AuthApi {
  final ApiClient _client;
  final TokenStorage _tokens;

  const AuthApi(this._client, this._tokens);

  static const _clientKind = 'mobile';

  /// Logs in and stores the returned tokens in secure storage. Runs with
  /// `authorize: false` (see [ApiClient]'s doc comment) — a wrong
  /// password's 401 must surface as `invalid_credentials`, not trigger a
  /// refresh attempt with no session to refresh.
  Future<void> login({required String email, required String password}) async {
    final data =
        await _client.send(
              '/v1/auth/login',
              method: 'POST',
              authorize: false,
              body: {
                'email': email,
                'password': password,
                'client': _clientKind,
              },
            )
            as JsonMap;
    await _tokens.saveTokens(
      accessToken: data['access_token'] as String,
      refreshToken: data['refresh_token'] as String,
    );
  }

  /// Calls the backend (best-effort — `crm_be`'s `logout` always answers
  /// 204 even for an already-invalid token, not-found-is-success, and a
  /// network failure here must never block the rest of this method) and
  /// always clears local tokens regardless of how that call went. A
  /// caller only needs to update UI state (`Session.markUnauthenticated`)
  /// after awaiting this — secure storage is already empty by the time
  /// it returns.
  Future<void> logout() async {
    final refreshToken = await _tokens.readRefreshToken();
    try {
      await _client.send(
        '/v1/auth/logout',
        method: 'POST',
        authorize: false,
        body: refreshToken != null ? {'refresh_token': refreshToken} : null,
      );
    } catch (_) {
      // Swallowed deliberately — see doc comment above.
    }
    await _tokens.clear();
  }

  Future<MeResult> me() async {
    final data = await _client.send('/v1/me') as JsonMap;
    return MeResult.fromJson(data);
  }
}
