import 'package:equatable/equatable.dart';

/// Domain-level errors — what a repository returns via
/// `Either<Failure, T>` (dartz), never `ApiError`/`SessionExpiredException`
/// directly. Those two live in `core/api_error.dart` and are a
/// data-layer/network concept (HTTP status, error envelope shape); the
/// domain layer must not know HTTP exists at all (Clean Architecture's
/// dependency rule: inner layers never depend on outer ones). Repository
/// implementations are what translate one into the other.
sealed class Failure extends Equatable {
  final String message;

  const Failure(this.message);

  @override
  List<Object?> get props => [message];
}

/// Wrong email/password, or any other rejected credential — the message
/// is already the user-facing Indonesian text from `ApiError.message`
/// (`crm_be`'s own error envelope, Aturan #33), not reworded here.
class InvalidCredentialsFailure extends Failure {
  const InvalidCredentialsFailure(super.message);
}

/// Refresh could not recover the session — TD §4.2's acceptance
/// criterion (membership deactivated, refresh token revoked/expired).
/// Local tokens are already cleared by the time this is thrown (`core`'s
/// `ApiClient`/`SecureTokenStorage`), so the only thing left to do with
/// this is show the login screen again.
class SessionExpiredFailure extends Failure {
  const SessionExpiredFailure(super.message);
}

/// No enrolled biometric, hardware error, or any other reason
/// `BiometricRepository` couldn't complete the check/prompt.
class BiometricFailure extends Failure {
  const BiometricFailure(super.message);
}

/// Anything else — a non-2xx response this app has no specific handling
/// for, or the request never reached the server at all (DNS, timeout,
/// connection refused).
class UnexpectedFailure extends Failure {
  const UnexpectedFailure(super.message);
}

/// Aturan #35 — `leads` and `tasks` both use optimistic locking
/// (`version`); a stale write is REJECTED, never silently overwritten,
/// and `crm_be`'s `409` body carries the row's current state so the UI
/// can show it and offer "muat ulang" (design brief §8.2). Generic over
/// [T] so both `leads` (#72) and, once #73 needs it, `tasks` share this
/// without `core/` importing either feature's entity — `core/` never
/// imports `features/`.
class VersionConflictFailure<T> extends Failure {
  final T current;

  const VersionConflictFailure(super.message, this.current);

  @override
  List<Object?> get props => [message, current];
}
