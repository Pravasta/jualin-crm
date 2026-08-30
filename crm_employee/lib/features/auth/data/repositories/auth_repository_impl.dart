import 'package:dartz/dartz.dart';

import '../../../../core/cache/response_cache.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/network/run_api_call.dart';
import '../../../../core/secure_store.dart';
import '../../domain/entities/auth_user.dart';
import '../../domain/repositories/auth_repository.dart';
import '../datasources/auth_remote_data_source.dart';

/// Coordinates `AuthRemoteDataSource` (the API), `TokenStorage` (secure
/// storage), and `ResponseCache` (offline cache, TD §7) — none of them
/// alone is enough to fulfil `login`/`logout`, which is exactly the kind
/// of orchestration a repository implementation exists for.
/// `ApiError`/`SessionExpiredException` (network layer,
/// `core/api_error.dart`) → `Failure` (domain layer,
/// `core/error/failures.dart`) translation is `runApiCall`'s job now
/// (#71) — extracted once `LeadRepositoryImpl` needed the identical
/// three-way catch this file used to write out by hand.
class AuthRepositoryImpl implements AuthRepository {
  final AuthRemoteDataSource remoteDataSource;
  final TokenStorage tokenStorage;
  final ResponseCache responseCache;

  const AuthRepositoryImpl({
    required this.remoteDataSource,
    required this.tokenStorage,
    required this.responseCache,
  });

  @override
  Future<bool> hasStoredSession() async {
    return await tokenStorage.readRefreshToken() != null;
  }

  @override
  Future<Either<Failure, void>> login({
    required String email,
    required String password,
  }) {
    return runApiCall(() async {
      final data = await remoteDataSource.login(
        email: email,
        password: password,
      );
      await tokenStorage.saveTokens(
        accessToken: data['access_token'] as String,
        refreshToken: data['refresh_token'] as String,
      );
    }, onApiError: (e) => e.code == 'invalid_credentials'
        ? InvalidCredentialsFailure(e.message)
        : UnexpectedFailure(e.message));
  }

  @override
  Future<Either<Failure, AuthUser>> getCurrentUser() {
    return runApiCall(remoteDataSource.getCurrentUser);
  }

  @override
  Future<Either<Failure, void>> logout() async {
    final refreshToken = await tokenStorage.readRefreshToken();
    try {
      await remoteDataSource.logout(refreshToken: refreshToken);
    } catch (_) {
      // Best-effort, deliberately swallowed — crm_be's own logout is
      // not-found-is-success, and a network failure here must never
      // block the local teardown below.
    }
    await tokenStorage.clear();
    // TD §7: cache holds one organization's lead/task data — a device
    // that switches users must never show the previous user's leftovers.
    await responseCache.clear();
    return const Right(null);
  }
}
