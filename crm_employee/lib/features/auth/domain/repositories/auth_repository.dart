import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../entities/auth_user.dart';

/// Declared by the domain, implemented by the data layer
/// (`AuthRepositoryImpl`) — the same "consumer owns the interface" shape
/// `crm_be`'s ADR-011 uses for `port.go`. `AuthBloc` (presentation) only
/// ever sees this interface, never `ApiClient`/`TokenStorage` directly.
abstract class AuthRepository {
  /// Reads secure storage only — no network call. True means "a refresh
  /// token is stored", not "it's still valid"; `getCurrentUser` is what
  /// actually proves that (TD §4.1: bootstrap check vs. the biometric
  /// gate that follows it are two separate steps).
  Future<bool> hasStoredSession();

  Future<Either<Failure, void>> login({
    required String email,
    required String password,
  });

  /// `GET /v1/me` — also what a fresh access token's validity gets
  /// proven through (a stale one triggers `ApiClient`'s transparent
  /// refresh underneath this call).
  Future<Either<Failure, AuthUser>> getCurrentUser();

  /// Best-effort against the backend, always clears local tokens
  /// regardless of the network outcome (TD's not-found-is-success
  /// reasoning, mirrored client-side).
  Future<Either<Failure, void>> logout();
}
