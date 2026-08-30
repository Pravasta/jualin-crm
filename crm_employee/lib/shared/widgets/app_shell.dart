import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../core/di/injection_container.dart';
import '../../features/auth/domain/entities/auth_user.dart';
import '../../features/auth/presentation/bloc/auth_bloc.dart';
import '../../features/auth/presentation/bloc/auth_event.dart';
import '../../features/auth/presentation/bloc/auth_state.dart';
import '../../features/leads/presentation/bloc/leads_bloc.dart';
import '../../features/leads/presentation/pages/leads_page.dart';
import '../nav.dart';
import '../theme.dart';
import 'placeholder_screen.dart';

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

  // Tugas Saya and Notifikasi are still placeholders — #73 replaces
  // both outright, per PlaceholderScreen's own doc comment. Lead Saya
  // is real as of #71. `late final` (not `static const`): LeadsBloc
  // comes from get_it, a runtime call, so this list can't be built at
  // compile time — but it's still only ever constructed once per
  // _AppShellState, the same lifetime `static const` gave the
  // placeholders.
  late final List<Widget> _tabBodies = [
    BlocProvider<LeadsBloc>(
      create: (_) => sl<LeadsBloc>(),
      child: const LeadsPage(),
    ),
    const PlaceholderScreen(
      title: 'Tugas Saya',
      description: 'Tugas yang dibuat untuk lead Anda akan tampil di sini.',
      issue: 73,
    ),
    const PlaceholderScreen(
      title: 'Notifikasi',
      description:
          'Pemberitahuan lead yang ditugaskan kepada Anda akan tampil di sini.',
      issue: 73,
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
          body: IndexedStack(index: _selectedIndex, children: _tabBodies),
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
