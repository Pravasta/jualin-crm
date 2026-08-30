import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import 'core/di/injection_container.dart';
import 'features/auth/presentation/bloc/auth_bloc.dart';
import 'features/auth/presentation/bloc/auth_event.dart';
import 'features/auth/presentation/pages/auth_gate_page.dart';
import 'shared/theme.dart';

class App extends StatelessWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocProvider<AuthBloc>(
      // sl<AuthBloc>() is a lazy singleton — this is the one place it's
      // ever resolved and given its first event; every screen below
      // reads the SAME instance via context.read<AuthBloc>()/BlocBuilder.
      create: (_) => sl<AuthBloc>()..add(const AuthAppStarted()),
      child: MaterialApp(
        title: 'Jualin CRM',
        debugShowCheckedModeBanner: false,
        theme: AppTheme.light,
        home: const AuthGatePage(),
      ),
    );
  }
}
