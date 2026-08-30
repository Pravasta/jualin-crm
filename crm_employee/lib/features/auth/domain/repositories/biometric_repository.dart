/// Separate from `AuthRepository` deliberately — biometric auth talks to
/// the device OS (`local_auth`), never the network, a genuinely different
/// data source than everything else this feature does.
abstract class BiometricRepository {
  /// False for two different reasons acceptance criterion #6/#7 treat
  /// identically: no biometric hardware, or hardware with nothing
  /// enrolled. Never throws — a platform error here also means "no".
  Future<bool> canAuthenticate();

  /// True only on an explicit successful match. False for a cancel, a
  /// failed match, or a platform error — never a reason to let the user
  /// in anyway.
  Future<bool> authenticate();
}
