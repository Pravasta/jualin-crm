import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../bloc/auth_bloc.dart';
import '../bloc/auth_state.dart';
import 'biometric_gate_page.dart';
import 'home_page.dart';
import 'login_page.dart';

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
          AuthAuthenticated() => const HomePage(),
        };
      },
    );
  }
}

class _SplashPage extends StatelessWidget {
  const _SplashPage();

  @override
  Widget build(BuildContext context) {
    return const Scaffold(body: Center(child: CircularProgressIndicator()));
  }
}
