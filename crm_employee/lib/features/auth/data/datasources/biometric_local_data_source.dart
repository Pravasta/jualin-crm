import 'package:local_auth/local_auth.dart';

/// Wraps `local_auth` directly — no meaningful behavior in `flutter
/// test`'s host environment (no real device biometrics to query), which
/// is why `BiometricRepositoryImpl` sits between this and the domain
/// layer: tests exercise the repository against a fake data source.
abstract class BiometricLocalDataSource {
  Future<bool> canCheckBiometrics();
  Future<List<BiometricType>> getAvailableBiometrics();
  Future<bool> authenticate();
}

class BiometricLocalDataSourceImpl implements BiometricLocalDataSource {
  final LocalAuthentication _localAuth;

  BiometricLocalDataSourceImpl({LocalAuthentication? localAuth})
    : _localAuth = localAuth ?? LocalAuthentication();

  @override
  Future<bool> canCheckBiometrics() => _localAuth.canCheckBiometrics;

  @override
  Future<List<BiometricType>> getAvailableBiometrics() =>
      _localAuth.getAvailableBiometrics();

  @override
  Future<bool> authenticate() {
    return _localAuth.authenticate(
      localizedReason: 'Konfirmasi identitas Anda untuk membuka Jualin CRM',
      biometricOnly: true,
      persistAcrossBackgrounding: true,
    );
  }
}
