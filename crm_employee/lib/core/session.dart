import 'package:flutter/foundation.dart';

import 'secure_store.dart';

enum SessionStatus {
  /// Not yet checked secure storage — the app's very first frame.
  unknown,

  /// A refresh token is present in secure storage. This means "has a
  /// stored session", not "passed biometric" — `AuthGate` is the layer
  /// that decides whether reaching this status is enough to show the
  /// rest of the app, or whether biometric must succeed first (TD §4.1).
  authenticated,

  /// No stored session, or one that just ended (logout, or refresh could
  /// not recover it — TD §4.2).
  unauthenticated,
}

/// Session state shared across the app — `ChangeNotifier` + `provider`
/// (TD §6, decision M6/§6): the smallest thing that solves "screens need
/// to know whether the user is logged in" without hand-rolling an
/// `InheritedWidget` for it (Aturan #27 — that would be MORE boilerplate
/// for the same result, not less).
///
/// Deliberately holds no reference to [AuthApi]/`ApiClient` — it never
/// makes a network call itself. `features/auth/` orchestrates the actual
/// login/logout HTTP calls and tells `Session` the outcome via the
/// methods below, keeping this file's only dependency the thing it
/// genuinely needs to read at startup: whether a token is already
/// stored.
class Session extends ChangeNotifier {
  final TokenStorage _tokens;

  // Not `required this._tokens`: a named parameter's label must be a
  // public identifier to be callable from another file (app.dart
  // constructs this), which a private field name never is.
  // ignore: prefer_initializing_formals
  Session({required TokenStorage tokens}) : _tokens = tokens;

  SessionStatus _status = SessionStatus.unknown;
  SessionStatus get status => _status;

  /// Reads whatever secure storage already has, without contacting the
  /// network — called once at startup by `AuthGate` before it decides
  /// whether to ask for biometric.
  Future<void> bootstrap() async {
    final refreshToken = await _tokens.readRefreshToken();
    _status = refreshToken != null
        ? SessionStatus.authenticated
        : SessionStatus.unauthenticated;
    notifyListeners();
  }

  /// Called after `AuthApi.login` has already stored fresh tokens.
  void markAuthenticated() {
    _status = SessionStatus.authenticated;
    notifyListeners();
  }

  /// Called after logout, and after `ApiClient` gives up on refresh
  /// (`SessionExpiredException` — TD §4.2's acceptance criterion). In
  /// both cases secure storage has already been cleared by the caller;
  /// this only updates the state screens observe so the app navigates
  /// back to login.
  void markUnauthenticated() {
    _status = SessionStatus.unauthenticated;
    notifyListeners();
  }
}
