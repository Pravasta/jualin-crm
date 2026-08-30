import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../core/di/injection_container.dart';
import '../../features/auth/domain/entities/auth_user.dart';
import '../../features/auth/presentation/bloc/auth_bloc.dart';
import '../../features/auth/presentation/bloc/auth_event.dart';
import '../../features/auth/presentation/bloc/auth_state.dart';
import '../../features/leads/presentation/bloc/leads_bloc.dart';
import '../../features/leads/presentation/open_lead_detail.dart';
import '../../features/leads/presentation/pages/leads_page.dart';
import '../../features/notifications/presentation/bloc/notifications_bloc.dart';
import '../../features/notifications/presentation/pages/notifications_page.dart';
import '../../features/push/presentation/bloc/push_bloc.dart';
import '../../features/push/presentation/bloc/push_event.dart';
import '../../features/push/presentation/bloc/push_state.dart';
import '../../features/tasks/presentation/bloc/tasks_bloc.dart';
import '../../features/tasks/presentation/pages/tasks_page.dart';
import '../nav.dart';
import '../theme.dart';

/// The frame every content screen sits inside (design brief §4, "Kerangka
/// aplikasi") — used for everything past login. Header (56dp): screen
/// title, static, left-aligned — never a button. Right: the ONLY account
/// entry point is the avatar, opening a bottom sheet with name/org/Keluar
/// — no hamburger menu, because there are only three destinations and
/// they all fit the bottom nav (68dp): Lead Saya, Tugas Saya, Notifikasi.
///
/// Detail Lead (#72) is reached by push from Lead Saya/Notifikasi, so it
/// is never one of the `IndexedStack` children here — it gets its own
/// `Navigator` push with a back button in the header position instead.
class AppShell extends StatefulWidget {
  const AppShell({super.key});

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
  int _selectedIndex = 0;

  static const _destinations = AppDestination.values;

  // `late final`: each bloc comes from get_it, a runtime call, so this
  // list can't be built at compile time — but it's still only ever
  // constructed once per _AppShellState.
  late final List<Widget> _tabBodies = [
    BlocProvider<LeadsBloc>(
      create: (_) => sl<LeadsBloc>(),
      child: const LeadsPage(),
    ),
    BlocProvider<TasksBloc>(
      create: (_) => sl<TasksBloc>(),
      child: const TasksPage(),
    ),
    BlocProvider<NotificationsBloc>(
      create: (_) => sl<NotificationsBloc>(),
      child: const NotificationsPage(),
    ),
  ];

  void _openAccountSheet(AuthUser user) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: AppColors.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.dialog),
        ),
      ),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.space20,
              AppSpacing.space12,
              AppSpacing.space20,
              AppSpacing.space20,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Center(
                  child: Container(
                    width: 36,
                    height: 4,
                    margin: const EdgeInsets.only(bottom: AppSpacing.space16),
                    decoration: BoxDecoration(
                      color: AppColors.border,
                      borderRadius: BorderRadius.circular(4),
                    ),
                  ),
                ),
                Text(user.fullName, style: AppTextStyles.cardTitle),
                const SizedBox(height: AppSpacing.space4),
                Text(
                  user.organizationName,
                  style: AppTextStyles.body.copyWith(
                    color: AppColors.mutedForeground,
                  ),
                ),
                const SizedBox(height: AppSpacing.space20),
                OutlinedButton(
                  onPressed: () {
                    Navigator.of(sheetContext).pop();
                    context.read<AuthBloc>().add(const AuthLogoutRequested());
                  },
                  style: OutlinedButton.styleFrom(
                    minimumSize: const Size.fromHeight(kMinTouchTarget),
                  ),
                  child: const Text('Keluar'),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final destination = _destinations[_selectedIndex];

    return BlocBuilder<AuthBloc, AuthState>(
      builder: (context, state) {
        // AppShell is only ever reached from AuthState.AuthAuthenticated
        // (see AuthGatePage) — this branch exists so the widget doesn't
        // crash for one frame during the logout transition, not because
        // it's a state this screen is meant to render normally.
        final user = state is AuthAuthenticated ? state.user : null;

        return Scaffold(
          appBar: AppBar(
            title: Text(
              navTitle(destination),
              style: AppTextStyles.screenTitle,
            ),
            actions: [
              if (user != null)
                Padding(
                  padding: const EdgeInsets.only(right: AppSpacing.space16),
                  child: GestureDetector(
                    onTap: () => _openAccountSheet(user),
                    child: CircleAvatar(
                      radius: 18,
                      backgroundColor: AppColors.surfaceSunken,
                      child: Text(
                        initialsOf(user.fullName),
                        style: const TextStyle(
                          color: AppColors.accentStrong,
                          fontSize: 13,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
          body: Column(
            children: [
              const ForegroundPushBanner(),
              Expanded(
                child: IndexedStack(index: _selectedIndex, children: _tabBodies),
              ),
            ],
          ),
          bottomNavigationBar: NavigationBar(
            selectedIndex: _selectedIndex,
            onDestinationSelected: (index) =>
                setState(() => _selectedIndex = index),
            backgroundColor: AppColors.surfaceSunken,
            indicatorColor: Colors.transparent,
            destinations: [
              for (final d in _destinations)
                NavigationDestination(
                  icon: Icon(
                    navIcon(d),
                    color: d == destination
                        ? AppColors.accentStrong
                        : AppColors.mutedForeground,
                  ),
                  label: navTitle(d),
                ),
            ],
          ),
        );
      },
    );
  }
}

/// Design brief §10 / TD §10 — a push that arrives while the app is
/// already open shows a banner here, INSIDE whatever tab the user is
/// already on; it never navigates by itself (that's what makes it
/// different from a background/cold-start tap, which does). Tapping the
/// banner itself is the one exception — an explicit gesture, same as
/// tapping the system tray notification would be.
///
/// Public (not `_ForegroundPushBanner`) specifically so a widget test
/// can pump it directly against a mocked `PushBloc`, instead of the
/// alternative — building the whole `AppShell` (its own DI-resolved
/// `AuthBloc`/`LeadsBloc`/`TasksBloc`/`NotificationsBloc` for one
/// widget's render coverage) — just to prove this one renders.
class ForegroundPushBanner extends StatelessWidget {
  const ForegroundPushBanner({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocBuilder<PushBloc, PushState>(
      builder: (context, state) {
        final message = state.foregroundMessage;
        if (message == null) return const SizedBox.shrink();

        return Material(
          color: AppColors.surface,
          child: InkWell(
            onTap: () {
              context.read<PushBloc>().add(
                const PushForegroundBannerDismissed(),
              );
              if (message.leadId != null) {
                openLeadDetail(context, message.leadId!);
              }
            },
            child: Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.space16),
              decoration: const BoxDecoration(
                border: Border(bottom: BorderSide(color: AppColors.border)),
              ),
              child: Row(
                children: [
                  const Icon(
                    Icons.notifications_active,
                    color: AppColors.primary,
                    size: 20,
                  ),
                  const SizedBox(width: AppSpacing.space12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (message.title != null)
                          Text(message.title!, style: AppTextStyles.cardTitle),
                        if (message.body != null)
                          Text(message.body!, style: AppTextStyles.body),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, size: 18),
                    color: AppColors.mutedForeground,
                    onPressed: () => context.read<PushBloc>().add(
                      const PushForegroundBannerDismissed(),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
