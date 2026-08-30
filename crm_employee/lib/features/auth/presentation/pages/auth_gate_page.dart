import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/theme.dart';
import '../../../../shared/widgets/app_shell.dart';
import '../bloc/auth_bloc.dart';
import '../bloc/auth_state.dart';
import 'biometric_gate_page.dart';
import 'login_page.dart';
import 'session_expired_page.dart';

/// Picks a page purely from `AuthState`'s type — `AuthBloc` (already
/// dispatched `AuthAppStarted` where this is provided, see
/// `injection_container.dart`/`app.dart`) owns every transition; this
/// widget has no state or logic of its own.
class AuthGatePage extends StatelessWidget {
  const AuthGatePage({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocBuilder<AuthBloc, AuthState>(
      builder: (context, state) {
        return switch (state) {
          AuthInitial() || AuthChecking() => const _SplashPage(),
          AuthNeedsBiometric() => const BiometricGatePage(),
          AuthNeedsPassword() => const LoginPage(),
          AuthSessionExpired() => const SessionExpiredPage(),
          AuthAuthenticated() => const AppShell(),
        };
      },
    );
  }
}

class _SplashPage extends StatelessWidget {
  const _SplashPage();

  @override
  Widget build(BuildContext context) {
    return const Scaffold(
      backgroundColor: AppColors.surface,
      body: Center(child: CircularProgressIndicator(color: AppColors.primary)),
    );
  }
}
