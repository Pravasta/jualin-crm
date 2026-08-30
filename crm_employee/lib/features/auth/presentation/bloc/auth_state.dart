import 'package:equatable/equatable.dart';

import '../../domain/entities/auth_user.dart';

sealed class AuthState extends Equatable {
  const AuthState();

  @override
  List<Object?> get props => [];
}

/// Before `AuthAppStarted` has been handled at all — the very first
/// frame, never actually rendered (the bloc moves straight to
/// `AuthChecking` on creation's first event).
class AuthInitial extends AuthState {
  const AuthInitial();
}

/// Reading secure storage / checking biometric availability — no
/// network call yet.
class AuthChecking extends AuthState {
  const AuthChecking();
}

/// A stored session exists and biometric hardware has something
/// enrolled — waiting on (or retrying) the OS prompt.
class AuthNeedsBiometric extends AuthState {
  final String? error;

  const AuthNeedsBiometric({this.error});

  @override
  List<Object?> get props => [error];
}

/// No stored session, biometric unavailable/not enrolled (falls through
/// explicitly — acceptance criterion #6), a login attempt in progress,
/// or one that just failed.
class AuthNeedsPassword extends AuthState {
  final bool isSubmitting;
  final String? error;

  const AuthNeedsPassword({this.isSubmitting = false, this.error});

  @override
  List<Object?> get props => [isSubmitting, error];
}

class AuthAuthenticated extends AuthState {
  final AuthUser user;

  const AuthAuthenticated(this.user);

  @override
  List<Object?> get props => [user];
}
