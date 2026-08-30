import '../../domain/repositories/biometric_repository.dart';
import '../datasources/biometric_local_data_source.dart';

class BiometricRepositoryImpl implements BiometricRepository {
  final BiometricLocalDataSource dataSource;

  const BiometricRepositoryImpl(this.dataSource);

  @override
  Future<bool> canAuthenticate() async {
    try {
      // canCheckBiometrics answers "does the hardware support this at
      // all" — not enrollment. Acceptance criterion #6 is specifically
      // about ENROLLED biometrics ("perangkat tanpa biometric
      // terdaftar"), so getAvailableBiometrics (local_auth's own doc
      // comment says it reflects what's actually usable right now) is
      // what decides — not isDeviceSupported, which is also true for a
      // device with only a PIN/pattern fallback and zero biometrics.
      final canCheck = await dataSource.canCheckBiometrics();
      if (!canCheck) return false;
      final enrolled = await dataSource.getAvailableBiometrics();
      return enrolled.isNotEmpty;
    } catch (_) {
      // A platform error here means "cannot verify support" — treated
      // as "no", the same conservative direction as a failed
      // authenticate().
      return false;
    }
  }

  @override
  Future<bool> authenticate() async {
    try {
      return await dataSource.authenticate();
    } catch (_) {
      // local_auth 3.x returns false for a plain failed match, but
      // throws for everything else — including the user cancelling.
      // Both outcomes mean the same thing here: did not authenticate.
      return false;
    }
  }
}
