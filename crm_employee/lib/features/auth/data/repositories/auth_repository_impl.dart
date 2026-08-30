import 'package:dartz/dartz.dart';

import '../../../../core/api_error.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/secure_store.dart';
import '../../domain/entities/auth_user.dart';
import '../../domain/repositories/auth_repository.dart';
import '../datasources/auth_remote_data_source.dart';

/// Coordinates `AuthRemoteDataSource` (the API) and `TokenStorage` (secure
/// storage) — either one alone is not enough to fulfil `login`/`logout`,
/// which is exactly the kind of orchestration a repository implementation
/// exists for. Translates `ApiError`/`SessionExpiredException` (network
/// layer, `core/api_error.dart`) into `Failure` (domain layer,
/// `core/error/failures.dart`) — the one place in this feature where
/// those two vocabularies meet.
class AuthRepositoryImpl implements AuthRepository {
  final AuthRemoteDataSource remoteDataSource;
  final TokenStorage tokenStorage;

  const AuthRepositoryImpl({
    required this.remoteDataSource,
    required this.tokenStorage,
  });

  @override
  Future<bool> hasStoredSession() async {
    return await tokenStorage.readRefreshToken() != null;
  }

  @override
  Future<Either<Failure, void>> login({
    required String email,
    required String password,
  }) async {
    try {
      final data = await remoteDataSource.login(
        email: email,
        password: password,
      );
      await tokenStorage.saveTokens(
        accessToken: data['access_token'] as String,
        refreshToken: data['refresh_token'] as String,
      );
      return const Right(null);
    } on ApiError catch (e) {
      if (e.code == 'invalid_credentials') {
        return Left(InvalidCredentialsFailure(e.message));
      }
      return Left(UnexpectedFailure(e.message));
    } catch (_) {
      return const Left(
        UnexpectedFailure(
          'Tidak dapat terhubung ke server. Periksa koneksi Anda.',
        ),
      );
    }
  }

  @override
  Future<Either<Failure, AuthUser>> getCurrentUser() async {
    try {
      final user = await remoteDataSource.getCurrentUser();
      return Right(user);
    } on SessionExpiredException catch (e) {
      return Left(SessionExpiredFailure(e.toString()));
    } on ApiError catch (e) {
      return Left(UnexpectedFailure(e.message));
    } catch (_) {
      return const Left(
        UnexpectedFailure(
          'Tidak dapat terhubung ke server. Periksa koneksi Anda.',
        ),
      );
    }
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
    return const Right(null);
  }
}
