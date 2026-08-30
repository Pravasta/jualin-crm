import 'package:equatable/equatable.dart';

sealed class AuthEvent extends Equatable {
  const AuthEvent();

  @override
  List<Object?> get props => [];
}

/// Dispatched once, at app launch — kicks off the bootstrap check +
/// biometric gate flow (TD §4.1).
class AuthAppStarted extends AuthEvent {
  const AuthAppStarted();
}

class AuthLoginSubmitted extends AuthEvent {
  final String email;
  final String password;

  const AuthLoginSubmitted({required this.email, required this.password});

  @override
  List<Object?> get props => [email, password];
}

/// The biometric gate's "Coba lagi" button, and re-entry after
/// `AuthUsePasswordRequested` is not taken.
class AuthBiometricRetryRequested extends AuthEvent {
  const AuthBiometricRetryRequested();
}

/// The biometric gate's "Masuk dengan password" fallback (acceptance
/// criterion #6's required exit from a failed/unavailable biometric
/// prompt).
class AuthUsePasswordRequested extends AuthEvent {
  const AuthUsePasswordRequested();
}

class AuthLogoutRequested extends AuthEvent {
  const AuthLogoutRequested();
}

/// Dispatched by ANY feature — not just the auth flow itself — the
/// moment its own repository call surfaces a `SessionExpiredFailure`.
/// Design brief §10: the Sesi Berakhir screen can appear "di layar mana
/// pun", not only right after biometric/login. `AuthBloc` is a
/// `registerLazySingleton` (`injection_container.dart`), so any other
/// bloc reaches it via `sl<AuthBloc>()` and adds this — the same
/// `Failure` vocabulary (`core/error/failures.dart`) every feature
/// already shares, just routed to the one bloc that owns navigating back
/// to login. First added for #71 (`LeadsBloc`), when a feature other
/// than auth itself first made its own authenticated API calls.
class AuthSessionInvalidated extends AuthEvent {
  const AuthSessionInvalidated();
}
