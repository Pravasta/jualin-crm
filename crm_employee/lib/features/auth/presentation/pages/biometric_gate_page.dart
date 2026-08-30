import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/theme.dart';
import '../bloc/auth_bloc.dart';
import '../bloc/auth_event.dart';
import '../bloc/auth_state.dart';

/// Design brief §5 — "Gerbang biometric" (waiting/retry) and "Biometric
/// gagal" states, one widget switching its icon/copy on `error`.
///
/// The design shows the last-known user's name/organization on this
/// screen ("Budi Santoso" / "Toko Sinar Jaya") — not shown here. Nothing
/// in this app caches a user profile locally yet; the only local cache
/// TD §7 describes is lead/task data (#71), not identity, and fetching
/// the profile from the network before biometric succeeds would defeat
/// the point of gating on it first. Generic copy instead — revisit if a
/// local profile cache is ever built for another reason.
class BiometricGatePage extends StatelessWidget {
  const BiometricGatePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.space32),
            child: BlocBuilder<AuthBloc, AuthState>(
              builder: (context, state) {
                final error = state is AuthNeedsBiometric ? state.error : null;
                final failed = error != null;

                return Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Container(
                      width: 88,
                      height: 88,
                      decoration: BoxDecoration(
                        color: failed
                            ? AppColors.dangerTint
                            : const Color(0xFFFDF0E8),
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        failed ? Icons.fingerprint : Icons.fingerprint,
                        size: 40,
                        color: failed
                            ? AppColors.danger
                            : AppColors.accentStrong,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.space24),
                    Text(
                      failed ? 'Verifikasi gagal' : 'Verifikasi untuk masuk',
                      style: AppTextStyles.cardTitle,
                    ),
                    const SizedBox(height: AppSpacing.space8),
                    Text(
                      error ?? 'Buka kembali dengan biometrik Anda.',
                      textAlign: TextAlign.center,
                      style: AppTextStyles.body.copyWith(
                        color: AppColors.mutedForeground,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.space32),
                    if (failed)
                      SizedBox(
                        width: double.infinity,
                        child: FilledButton(
                          onPressed: () => context.read<AuthBloc>().add(
                            const AuthBiometricRetryRequested(),
                          ),
                          child: const Text('Coba lagi'),
                        ),
                      ),
                    const SizedBox(height: AppSpacing.space12),
                    TextButton(
                      onPressed: () => context.read<AuthBloc>().add(
                        const AuthUsePasswordRequested(),
                      ),
                      style: TextButton.styleFrom(
                        foregroundColor: AppColors.accentStrong,
                        textStyle: const TextStyle(fontWeight: FontWeight.w600),
                      ),
                      child: const Text('Masuk dengan kata sandi'),
                    ),
                  ],
                );
              },
            ),
          ),
        ),
      ),
    );
  }
}
