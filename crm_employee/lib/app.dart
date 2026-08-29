import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'core/api_client.dart';
import 'core/secure_store.dart';
import 'core/session.dart';
import 'features/auth/auth_api.dart';
import 'features/auth/auth_gate.dart';
import 'features/auth/biometric_authenticator.dart';

/// Composition root — every service is constructed exactly once here and
/// handed down through `provider`, the same shape `cmd/api/main.go` wires
/// `crm_be`'s dependencies at boot (ADR-011's composition root, applied to
/// a Flutter app instead of a Go binary).
class App extends StatelessWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: [
        Provider<TokenStorage>(create: (_) => const SecureTokenStorage()),
        Provider<ApiClient>(
          create: (context) => ApiClient(tokens: context.read<TokenStorage>()),
        ),
        Provider<AuthApi>(
          create: (context) => AuthApi(
            context.read<ApiClient>(),
            context.read<TokenStorage>(),
          ),
        ),
        Provider<BiometricAuthenticator>(
          create: (_) => LocalAuthBiometricAuthenticator(),
        ),
        ChangeNotifierProvider<Session>(
          create: (context) => Session(tokens: context.read<TokenStorage>()),
        ),
      ],
      child: MaterialApp(
        title: 'Jualin CRM',
        debugShowCheckedModeBanner: false,
        // Token/theme comes from Claude Design's output — issue #70.
        // Deliberately unstyled here (issue #69's own boundary).
        theme: ThemeData(useMaterial3: true, colorSchemeSeed: Colors.blue),
        home: const AuthGate(),
      ),
    );
  }
}
