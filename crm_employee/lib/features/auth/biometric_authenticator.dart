import 'package:local_auth/local_auth.dart';

/// `AuthGate` depends on this interface, not `LocalAuthentication`
/// directly — `local_auth` has no meaningful behavior in `flutter test`'s
/// host environment (no real device biometrics to query), so tests inject
/// a fake instead.
abstract class BiometricAuthenticator {
  /// False for two different reasons that acceptance criterion #6/#7
  /// treat identically: the device has no biometric hardware, or it has
  /// hardware but nothing enrolled. Either way `AuthGate` must fall
  /// through to the password screen explicitly, never skip the check.
  Future<bool> get canAuthenticate;

  /// True only on an explicit successful biometric match. False for a
  /// cancel, a failed match, or any platform error — `AuthGate` treats
  /// all of these as "did not authenticate", never as a reason to let
  /// the user in anyway.
  Future<bool> authenticate();
}

class LocalAuthBiometricAuthenticator implements BiometricAuthenticator {
  final LocalAuthentication _localAuth;

  LocalAuthBiometricAuthenticator({LocalAuthentication? localAuth})
    : _localAuth = localAuth ?? LocalAuthentication();

  @override
  Future<bool> get canAuthenticate async {
    try {
      // canCheckBiometrics answers "does the hardware support this at
      // all" — not enrollment. The acceptance criterion is specifically
      // about ENROLLED biometrics ("perangkat tanpa biometric
      // terdaftar"), so getAvailableBiometrics (which local_auth's own
      // doc comment says reflects what's actually usable right now) is
      // what decides, not isDeviceSupported — that one also returns true
      // when only a PIN/pattern fallback exists with zero biometrics
      // enrolled, which would defeat the point of biometricOnly below.
      final canCheck = await _localAuth.canCheckBiometrics;
      if (!canCheck) return false;
      final enrolled = await _localAuth.getAvailableBiometrics();
      return enrolled.isNotEmpty;
    } catch (_) {
      // A platform error here means "cannot verify support" — treated as
      // "no", the same conservative direction as a failed authenticate().
      return false;
    }
  }

  @override
  Future<bool> authenticate() async {
    try {
      return await _localAuth.authenticate(
        localizedReason: 'Konfirmasi identitas Anda untuk membuka Jualin CRM',
        biometricOnly: true,
        persistAcrossBackgrounding: true,
      );
    } catch (_) {
      // local_auth 3.x returns false for a plain failed match, but
      // throws LocalAuthException for everything else — including the
      // user cancelling. Both outcomes mean the same thing to AuthGate:
      // did not authenticate.
      return false;
    }
  }
}
