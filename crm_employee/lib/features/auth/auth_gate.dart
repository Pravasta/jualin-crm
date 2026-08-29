import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../core/session.dart';
import 'biometric_authenticator.dart';
import 'login_screen.dart';
import 'placeholder_home_screen.dart';

/// Whatever biometric has (or hasn't) been satisfied THIS app open —
/// deliberately separate from [SessionStatus], which only answers "is
/// there a stored token" and knows nothing about biometric at all (TD
/// §4.1's flow is a UI-level concern layered on top of session state, not
/// part of session state itself).
enum _Stage { checking, needsBiometric, needsPassword, ready }

/// The root gate every app open passes through (TD §4.1):
///
/// ```
/// no stored session   -> LoginScreen
/// stored session
///   no biometric enrolled -> LoginScreen (falls through explicitly,
///                             never skipped — acceptance criterion)
///   biometric available   -> prompt
///     success -> home
///     fail/cancel -> "Autentikasi gagal" + retry + "Masuk dengan password"
/// ```
class AuthGate extends StatefulWidget {
  const AuthGate({super.key});

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  _Stage _stage = _Stage.checking;
  String? _biometricError;

  @override
  void initState() {
    super.initState();
    unawaited(_bootstrap());
  }

  Future<void> _bootstrap() async {
    final session = context.read<Session>();
    await session.bootstrap();
    if (!mounted) return;

    if (session.status != SessionStatus.authenticated) {
      setState(() => _stage = _Stage.needsPassword);
      return;
    }
    await _attemptBiometric();
  }

  Future<void> _attemptBiometric() async {
    final biometric = context.read<BiometricAuthenticator>();

    if (!await biometric.canAuthenticate) {
      if (!mounted) return;
      setState(() => _stage = _Stage.needsPassword);
      return;
    }

    if (!mounted) return;
    setState(() {
      _stage = _Stage.needsBiometric;
      _biometricError = null;
    });

    final ok = await biometric.authenticate();
    if (!mounted) return;
    setState(() {
      if (ok) {
        _stage = _Stage.ready;
      } else {
        _stage = _Stage.needsBiometric;
        _biometricError = 'Autentikasi biometrik gagal atau dibatalkan.';
      }
    });
  }

  /// Shared by the logout button and a session that expired mid-use
  /// (`PlaceholderHomeScreen`'s `onSessionEnded`) — both mean the same
  /// thing here: forget this app-open's biometric pass, go back to
  /// requiring password.
  void _onSessionEnded() {
    context.read<Session>().markUnauthenticated();
    setState(() => _stage = _Stage.needsPassword);
  }

  void _onPasswordLoginSuccess() {
    setState(() => _stage = _Stage.ready);
  }

  @override
  Widget build(BuildContext context) {
    switch (_stage) {
      case _Stage.checking:
        return const _SplashScreen();
      case _Stage.needsBiometric:
        return _BiometricScreen(
          error: _biometricError,
          onRetry: _attemptBiometric,
          onUsePassword: () => setState(() => _stage = _Stage.needsPassword),
        );
      case _Stage.needsPassword:
        return LoginScreen(onSuccess: _onPasswordLoginSuccess);
      case _Stage.ready:
        return PlaceholderHomeScreen(onSessionEnded: _onSessionEnded);
    }
  }
}

class _SplashScreen extends StatelessWidget {
  const _SplashScreen();

  @override
  Widget build(BuildContext context) {
    return const Scaffold(body: Center(child: CircularProgressIndicator()));
  }
}

class _BiometricScreen extends StatelessWidget {
  final String? error;
  final VoidCallback onRetry;
  final VoidCallback onUsePassword;

  const _BiometricScreen({
    required this.error,
    required this.onRetry,
    required this.onUsePassword,
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.fingerprint, size: 64),
                const SizedBox(height: 16),
                Text(error ?? 'Buka kembali dengan biometrik Anda.'),
                const SizedBox(height: 24),
                FilledButton(onPressed: onRetry, child: const Text('Coba lagi')),
                const SizedBox(height: 8),
                TextButton(
                  onPressed: onUsePassword,
                  child: const Text('Masuk dengan password'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
