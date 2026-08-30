import '../repositories/biometric_repository.dart';

/// Same reasoning as `CheckStoredSessionUseCase` — a plain boolean
/// result (true only on an explicit successful match), not
/// `Either`-wrapped.
class AuthenticateWithBiometricsUseCase {
  final BiometricRepository repository;

  const AuthenticateWithBiometricsUseCase(this.repository);

  Future<bool> call() => repository.authenticate();
}
