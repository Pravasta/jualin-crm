import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/theme.dart';
import '../bloc/auth_bloc.dart';
import '../bloc/auth_event.dart';

/// Design brief §10 — "muncul di layar mana pun saat refresh token
/// ditolak". Reached only from `AuthBloc`'s `AuthSessionExpired` state
/// (TD §4.2's acceptance criterion), never navigated to directly.
class SessionExpiredPage extends StatelessWidget {
  const SessionExpiredPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.space32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 72,
                  height: 72,
                  decoration: const BoxDecoration(
                    color: AppColors.surfaceSunken,
                    shape: BoxShape.circle,
                  ),
                  child: const Icon(
                    Icons.lock_outline,
                    size: 30,
                    color: AppColors.mutedForeground,
                  ),
                ),
                const SizedBox(height: AppSpacing.space24),
                const Text(
                  'Sesi Anda berakhir',
                  style: AppTextStyles.cardTitle,
                ),
                const SizedBox(height: AppSpacing.space8),
                Text(
                  'Untuk keamanan, masuk kembali untuk melanjutkan. '
                  'Data yang sudah tersimpan tidak hilang.',
                  textAlign: TextAlign.center,
                  style: AppTextStyles.body.copyWith(
                    color: AppColors.mutedForeground,
                  ),
                ),
                const SizedBox(height: AppSpacing.space32),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton(
                    onPressed: () => context.read<AuthBloc>().add(
                      const AuthUsePasswordRequested(),
                    ),
                    child: const Text('Masuk kembali'),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
