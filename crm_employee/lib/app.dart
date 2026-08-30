import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import 'core/di/injection_container.dart';
import 'features/auth/presentation/bloc/auth_bloc.dart';
import 'features/auth/presentation/bloc/auth_event.dart';
import 'features/auth/presentation/bloc/auth_state.dart';
import 'features/auth/presentation/pages/auth_gate_page.dart';
import 'features/leads/presentation/open_lead_detail.dart';
import 'features/push/presentation/bloc/push_bloc.dart';
import 'features/push/presentation/bloc/push_event.dart';
import 'features/push/presentation/bloc/push_state.dart';
import 'shared/theme.dart';

class App extends StatelessWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context) {
    return MultiBlocProvider(
      providers: [
        // sl<AuthBloc>()/sl<PushBloc>() are lazy singletons — this is
        // the one place each is ever resolved and given its first
        // event; every screen below reads the SAME instances via
        // context.read/BlocBuilder.
        BlocProvider<AuthBloc>(
          create: (_) => sl<AuthBloc>()..add(const AuthAppStarted()),
        ),
        BlocProvider<PushBloc>(
          create: (_) => sl<PushBloc>()..add(const PushInitialized()),
        ),
      ],
      child: MaterialApp(
        title: 'Jualin CRM',
        debugShowCheckedModeBanner: false,
        theme: AppTheme.light,
        home: const _DeeplinkAwareGate(),
      ),
    );
  }
}

/// Wraps `AuthGatePage` with the ONE place TD §10's deeplink is actually
/// consumed — `AuthBloc` and `PushBloc` coordinate purely through this
/// listener, neither bloc knows the other exists (same "presentation
/// coordinating with presentation" discipline `LeadsBloc.authBloc`/
/// `LeadDetailBloc.authBloc` already established, just with two
/// listeners instead of one bloc dispatching to another).
///
/// Checked from BOTH directions on purpose — TD §10's required case:
/// "notifikasi ditekan saat belum login → tujuan disimpan, dibuka
/// setelah login". If the push arrives/gets tapped first,
/// `PushBloc`'s own listener below fires once `AuthBloc` is already
/// `AuthAuthenticated`. If login finishes first and a `pendingLeadId`
/// was already sitting there (tapped before this listener ever ran, or
/// resolved by `PushInitialized`'s cold-start check before auth
/// settled), `AuthBloc`'s listener below catches it instead. Either
/// order works, and both funnel through the same `_maybeOpenDeeplink`
/// so there's exactly one place that decides "is it time to navigate".
class _DeeplinkAwareGate extends StatelessWidget {
  const _DeeplinkAwareGate();

  @override
  Widget build(BuildContext context) {
    return MultiBlocListener(
      listeners: [
        BlocListener<AuthBloc, AuthState>(
          listenWhen: (previous, current) => current is AuthAuthenticated,
          listener: (context, state) {
            context.read<PushBloc>().add(const PushRegistrationRequested());
            _maybeOpenDeeplink(context);
          },
        ),
        BlocListener<PushBloc, PushState>(
          listenWhen: (previous, current) =>
              current.pendingLeadId != null &&
              current.pendingLeadId != previous.pendingLeadId,
          listener: (context, state) => _maybeOpenDeeplink(context),
        ),
      ],
      child: const AuthGatePage(),
    );
  }

  void _maybeOpenDeeplink(BuildContext context) {
    final authState = context.read<AuthBloc>().state;
    final pushBloc = context.read<PushBloc>();
    final leadId = pushBloc.state.pendingLeadId;
    if (authState is! AuthAuthenticated || leadId == null) return;

    pushBloc.add(const PushDeeplinkConsumed());
    openLeadDetail(context, leadId);
  }
}
