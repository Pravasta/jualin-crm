import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../../domain/usecases/authenticate_with_biometrics_usecase.dart';
import '../../domain/usecases/check_biometric_availability_usecase.dart';
import '../../domain/usecases/check_stored_session_usecase.dart';
import '../../domain/usecases/get_current_user_usecase.dart';
import '../../domain/usecases/login_usecase.dart';
import '../../domain/usecases/logout_usecase.dart';
import 'auth_event.dart';
import 'auth_state.dart';

/// The whole app-open flow (TD §4.1) as a state machine:
///
/// ```
/// AuthAppStarted
///   no stored session        -> AuthNeedsPassword
///   stored session
///     no biometric enrolled  -> AuthNeedsPassword (falls through
///                                explicitly, never skipped)
///     biometric available    -> AuthNeedsBiometric
///       success               -> AuthAuthenticated
///       fail/cancel           -> AuthNeedsBiometric(error: ...)
/// ```
///
/// One bloc for the whole app lifetime (registered as a lazy singleton in
/// `injection_container.dart`), not one per page — the same "single
/// source of truth for session state" role `core/session.dart`'s
/// `ChangeNotifier` played before this rewrite.
class AuthBloc extends Bloc<AuthEvent, AuthState> {
  final CheckStoredSessionUseCase checkStoredSession;
  final CheckBiometricAvailabilityUseCase checkBiometricAvailability;
  final AuthenticateWithBiometricsUseCase authenticateWithBiometrics;
  final LoginUseCase login;
  final LogoutUseCase logout;
  final GetCurrentUserUseCase getCurrentUser;

  AuthBloc({
    required this.checkStoredSession,
    required this.checkBiometricAvailability,
    required this.authenticateWithBiometrics,
    required this.login,
    required this.logout,
    required this.getCurrentUser,
  }) : super(const AuthInitial()) {
    on<AuthAppStarted>(_onAppStarted);
    on<AuthLoginSubmitted>(_onLoginSubmitted);
    on<AuthBiometricRetryRequested>(_onBiometricRetryRequested);
    on<AuthUsePasswordRequested>(_onUsePasswordRequested);
    on<AuthLogoutRequested>(_onLogoutRequested);
    on<AuthSessionInvalidated>(_onSessionInvalidated);
  }

  Future<void> _onAppStarted(
    AuthAppStarted event,
    Emitter<AuthState> emit,
  ) async {
    emit(const AuthChecking());
    final hasSession = await checkStoredSession();
    if (!hasSession) {
      emit(const AuthNeedsPassword());
      return;
    }
    await _attemptBiometricThenLoadUser(emit);
  }

  Future<void> _onBiometricRetryRequested(
    AuthBiometricRetryRequested event,
    Emitter<AuthState> emit,
  ) async {
    await _attemptBiometricThenLoadUser(emit);
  }

  Future<void> _attemptBiometricThenLoadUser(Emitter<AuthState> emit) async {
    final canAuthenticate = await checkBiometricAvailability();
    if (!canAuthenticate) {
      // Device has no biometric hardware, or nothing enrolled — falls
      // through to password explicitly, never skipped.
      emit(const AuthNeedsPassword());
      return;
    }

    emit(const AuthNeedsBiometric());
    final authenticated = await authenticateWithBiometrics();
    if (!authenticated) {
      emit(
        const AuthNeedsBiometric(
          error: 'Autentikasi biometrik gagal atau dibatalkan.',
        ),
      );
      return;
    }
    await _loadCurrentUser(emit);
  }

  void _onUsePasswordRequested(
    AuthUsePasswordRequested event,
    Emitter<AuthState> emit,
  ) {
    emit(const AuthNeedsPassword());
  }

  Future<void> _onLoginSubmitted(
    AuthLoginSubmitted event,
    Emitter<AuthState> emit,
  ) async {
    emit(const AuthNeedsPassword(isSubmitting: true));
    final result = await login(
      LoginParams(email: event.email, password: event.password),
    );
    await result.fold(
      (failure) async => emit(AuthNeedsPassword(error: failure.message)),
      (_) async => _loadCurrentUser(emit),
    );
  }

  Future<void> _onLogoutRequested(
    AuthLogoutRequested event,
    Emitter<AuthState> emit,
  ) async {
    await logout(const NoParams());
    emit(const AuthNeedsPassword());
  }

  /// A `SessionExpiredFailure` some OTHER feature's repository surfaced
  /// on its own (#71's `LeadsBloc` is the first) — jumps straight to the
  /// Sesi Berakhir screen exactly like `_loadCurrentUser`'s own
  /// `SessionExpiredFailure` branch does, wherever in the app the user
  /// currently is (design brief §10).
  void _onSessionInvalidated(
    AuthSessionInvalidated event,
    Emitter<AuthState> emit,
  ) {
    emit(const AuthSessionExpired());
  }

  /// `GET /v1/me` — also what proves a stored/just-refreshed access token
  /// actually works. A `SessionExpiredFailure` here (refresh itself
  /// failed — TD §4.2) shows the dedicated Sesi Berakhir screen (design
  /// brief §10 — this can happen mid-use, not just at app open, so it
  /// gets an explanation rather than silently dropping back to a blank
  /// login form); any other failure shows its message inline so the user
  /// isn't left staring at a blank screen either.
  Future<void> _loadCurrentUser(Emitter<AuthState> emit) async {
    final result = await getCurrentUser(const NoParams());
    result.fold((failure) {
      if (failure is SessionExpiredFailure) {
        emit(const AuthSessionExpired());
      } else {
        emit(AuthNeedsPassword(error: failure.message));
      }
    }, (user) => emit(AuthAuthenticated(user)));
  }
}
