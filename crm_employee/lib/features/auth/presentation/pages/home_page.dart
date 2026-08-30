import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../bloc/auth_bloc.dart';
import '../bloc/auth_event.dart';
import '../bloc/auth_state.dart';

/// Everything past login is out of scope for issue #69 ("Belum ada
/// daftar lead, tema, atau navigasi antarlayar") — #71 replaces this
/// page wholesale with My Leads. Only reachable from `AuthAuthenticated`,
/// which already carries the `AuthUser` `GetCurrentUserUseCase` fetched
/// to get here — no separate network call needed just to render this
/// page, unlike the pre-Bloc `PlaceholderHomeScreen`.
class HomePage extends StatelessWidget {
  const HomePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Jualin CRM')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: BlocBuilder<AuthBloc, AuthState>(
            builder: (context, state) {
              if (state is! AuthAuthenticated) {
                return const CircularProgressIndicator();
              }
              final user = state.user;
              return Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    user.fullName,
                    style: const TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(user.email),
                  const SizedBox(height: 4),
                  Text('${user.organizationName} · ${user.role}'),
                  const SizedBox(height: 32),
                  OutlinedButton(
                    onPressed: () => context.read<AuthBloc>().add(
                      const AuthLogoutRequested(),
                    ),
                    child: const Text('Keluar'),
                  ),
                ],
              );
            },
          ),
        ),
      ),
    );
  }
}
