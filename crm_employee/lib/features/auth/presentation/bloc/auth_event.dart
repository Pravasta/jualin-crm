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
