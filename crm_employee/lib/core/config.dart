/// Build-time configuration — mirrors `crm_dashboard`'s
/// `NEXT_PUBLIC_API_BASE_URL` pattern (a compile-time value, not a
/// runtime env read Dart doesn't have on mobile).
class AppConfig {
  AppConfig._();

  /// Override with `--dart-define=API_BASE_URL=http://<lan-ip>:8080` when
  /// running on a real device (a physical phone can't reach `localhost` —
  /// or `10.0.2.2` — on the development machine at all).
  ///
  /// The default targets the Android **emulator**: `10.0.2.2` is the
  /// emulator's own alias for the host machine's `localhost` (a real
  /// device has no such alias, which is why it needs the override above).
  static const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://10.0.2.2:8080',
  );
}
