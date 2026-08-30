import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../bloc/auth_bloc.dart';
import '../bloc/auth_event.dart';
import '../bloc/auth_state.dart';

class BiometricGatePage extends StatelessWidget {
  const BiometricGatePage({super.key});

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
                BlocBuilder<AuthBloc, AuthState>(
                  builder: (context, state) {
                    final error = state is AuthNeedsBiometric
                        ? state.error
                        : null;
                    return Text(error ?? 'Buka kembali dengan biometrik Anda.');
                  },
                ),
                const SizedBox(height: 24),
                FilledButton(
                  onPressed: () => context.read<AuthBloc>().add(
                    const AuthBiometricRetryRequested(),
                  ),
                  child: const Text('Coba lagi'),
                ),
                const SizedBox(height: 8),
                TextButton(
                  onPressed: () => context.read<AuthBloc>().add(
                    const AuthUsePasswordRequested(),
                  ),
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
