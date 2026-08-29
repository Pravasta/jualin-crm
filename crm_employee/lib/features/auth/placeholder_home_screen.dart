import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../core/api_error.dart';
import 'auth_api.dart';

/// Everything past login is out of scope for issue #69 ("Belum ada daftar
/// lead, tema, atau navigasi antarlayar") — #71 replaces this screen
/// wholesale with My Leads. It exists so login + biometric + refresh have
/// somewhere real to land and be verified: `GET /v1/me` on entry proves
/// the stored access token actually works (and exercises `ApiClient`'s
/// transparent refresh-on-401 if it happened to be expired), and the
/// logout button is what acceptance criterion #7's manual verification
/// (deactivate the membership, watch the next refresh fail) needs to
/// reach in the first place.
class PlaceholderHomeScreen extends StatefulWidget {
  /// Called once, either from the logout button or when `GET /v1/me`
  /// comes back as an unrecoverable session (`SessionExpiredException`) —
  /// the caller (`AuthGate`) is what actually updates `Session` and
  /// navigates back to the login screen; this widget only reports "the
  /// session ended", not what to do about it.
  final VoidCallback onSessionEnded;

  const PlaceholderHomeScreen({super.key, required this.onSessionEnded});

  @override
  State<PlaceholderHomeScreen> createState() => _PlaceholderHomeScreenState();
}

class _PlaceholderHomeScreenState extends State<PlaceholderHomeScreen> {
  MeResult? _me;
  String? _error;
  bool _loggingOut = false;

  @override
  void initState() {
    super.initState();
    unawaited(_loadMe());
  }

  Future<void> _loadMe() async {
    final authApi = context.read<AuthApi>();
    try {
      final me = await authApi.me();
      if (!mounted) return;
      setState(() => _me = me);
    } on SessionExpiredException {
      widget.onSessionEnded();
    } catch (_) {
      if (!mounted) return;
      setState(() => _error = 'Tidak dapat memuat data akun.');
    }
  }

  Future<void> _logout() async {
    setState(() => _loggingOut = true);
    final authApi = context.read<AuthApi>();
    await authApi.logout();
    widget.onSessionEnded();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Jualin CRM')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (_me != null) ...[
                Text(
                  _me!.fullName,
                  style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 4),
                Text(_me!.email),
                const SizedBox(height: 4),
                Text('${_me!.organizationName} · ${_me!.role}'),
              ] else if (_error != null)
                Text(_error!)
              else
                const CircularProgressIndicator(),
              const SizedBox(height: 32),
              OutlinedButton(
                onPressed: _loggingOut ? null : _logout,
                child: const Text('Keluar'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
