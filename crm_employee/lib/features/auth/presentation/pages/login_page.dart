import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../shared/theme.dart';
import '../bloc/auth_bloc.dart';
import '../bloc/auth_event.dart';
import '../bloc/auth_state.dart';

/// Design brief §5 (Login) — normal + kredensial salah states. Doubles
/// as the "Masuk dengan kata sandi" fallback the biometric gate and the
/// Sesi Berakhir screen both offer.
///
/// Not implemented from the design: the "backoff" state's live countdown
/// ("Coba lagi dalam 04:37") — that needs `Retry-After` propagated all
/// the way from `crm_be`'s rate limiter through `ApiClient`/`ApiError`,
/// neither of which carries response headers today. A rate-limited login
/// still surfaces `crm_be`'s own message via the same themed error banner
/// below, just without the ticking clock — noted as a deliberate gap in
/// `docs/issues/070-design-foundation.md`, not silently skipped.
class LoginPage extends StatefulWidget {
  const LoginPage({super.key});

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  void _submit() {
    if (!_formKey.currentState!.validate()) return;
    context.read<AuthBloc>().add(
      AuthLoginSubmitted(
        email: _emailController.text.trim(),
        password: _passwordController.text,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.space24),
            child: Form(
              key: _formKey,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const SizedBox(height: AppSpacing.space24 * 4),
                  Text(
                    'Jualin CRM',
                    style: AppTextStyles.screenTitle.copyWith(
                      fontSize: 24,
                      color: AppColors.accentStrong,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.space4),
                  Text(
                    'Masuk untuk melihat lead Anda',
                    style: AppTextStyles.body.copyWith(
                      color: AppColors.mutedForeground,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.space40),
                  const Text('Email', style: AppTextStyles.metadata),
                  const SizedBox(height: AppSpacing.space8),
                  TextFormField(
                    controller: _emailController,
                    keyboardType: TextInputType.emailAddress,
                    autocorrect: false,
                    style: AppTextStyles.body,
                    validator: (value) =>
                        (value == null || value.trim().isEmpty)
                        ? 'Email wajib diisi'
                        : null,
                  ),
                  const SizedBox(height: AppSpacing.space20),
                  const Text('Kata sandi', style: AppTextStyles.metadata),
                  const SizedBox(height: AppSpacing.space8),
                  TextFormField(
                    controller: _passwordController,
                    obscureText: true,
                    style: AppTextStyles.body,
                    validator: (value) => (value == null || value.isEmpty)
                        ? 'Kata sandi wajib diisi'
                        : null,
                    onFieldSubmitted: (_) => _submit(),
                  ),
                  BlocBuilder<AuthBloc, AuthState>(
                    builder: (context, state) {
                      final error = state is AuthNeedsPassword
                          ? state.error
                          : null;
                      if (error == null) return const SizedBox.shrink();
                      return Padding(
                        padding: const EdgeInsets.only(top: AppSpacing.space20),
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppSpacing.space16,
                            vertical: AppSpacing.space12,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.dangerTint,
                            border: Border.all(
                              color: AppColors.danger.withValues(alpha: 0.3),
                            ),
                            borderRadius: BorderRadius.circular(10),
                          ),
                          child: Text(
                            error,
                            style: const TextStyle(
                              color: AppColors.danger,
                              fontWeight: FontWeight.w600,
                              fontSize: 13,
                            ),
                          ),
                        ),
                      );
                    },
                  ),
                  const SizedBox(height: AppSpacing.space24),
                  BlocBuilder<AuthBloc, AuthState>(
                    builder: (context, state) {
                      final submitting =
                          state is AuthNeedsPassword && state.isSubmitting;
                      return FilledButton(
                        onPressed: submitting ? null : _submit,
                        child: submitting
                            ? const SizedBox(
                                height: 20,
                                width: 20,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                  color: Colors.white,
                                ),
                              )
                            : const Text('Masuk'),
                      );
                    },
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
