import '../../../../core/api_client.dart';
import '../models/auth_user_model.dart';

/// Talks to `/v1/auth/*` and `/v1/me` — nothing more. Does not touch
/// secure storage (that's `AuthRepositoryImpl`'s job, coordinating this
/// data source with `TokenStorage`) and does not swallow errors (logout's
/// best-effort semantics are a repository-level policy, not this layer's
/// concern) — `ApiError`/`SessionExpiredException` propagate as-is.
abstract class AuthRemoteDataSource {
  /// Returns the raw `{access_token, refresh_token, ...}` map — the
  /// repository is what turns that into stored tokens.
  Future<Map<String, dynamic>> login({
    required String email,
    required String password,
  });

  Future<AuthUserModel> getCurrentUser();

  Future<void> logout({String? refreshToken});
}

class AuthRemoteDataSourceImpl implements AuthRemoteDataSource {
  final ApiClient client;

  const AuthRemoteDataSourceImpl(this.client);

  // Always sends client: "mobile" (crm_be's ClientMobile) — the field
  // that makes the handler respond with tokens in the body instead of
  // Set-Cookie (TD §4).
  static const _clientKind = 'mobile';

  @override
  Future<Map<String, dynamic>> login({
    required String email,
    required String password,
  }) async {
    // authorize: false — see ApiClient's doc comment: a wrong password's
    // 401 must surface as invalid_credentials, not trigger a refresh
    // attempt with no session to refresh.
    final data = await client.send(
      '/v1/auth/login',
      method: 'POST',
      authorize: false,
      body: {'email': email, 'password': password, 'client': _clientKind},
    );
    return data as Map<String, dynamic>;
  }

  @override
  Future<AuthUserModel> getCurrentUser() async {
    final data = await client.send('/v1/me');
    return AuthUserModel.fromJson(data as Map<String, dynamic>);
  }

  @override
  Future<void> logout({String? refreshToken}) async {
    await client.send(
      '/v1/auth/logout',
      method: 'POST',
      authorize: false,
      body: refreshToken != null ? {'refresh_token': refreshToken} : null,
    );
  }
}
