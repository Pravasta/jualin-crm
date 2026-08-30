import '../repositories/biometric_repository.dart';

/// Same reasoning as `CheckStoredSessionUseCase` — a plain boolean query,
/// not `Either`-wrapped.
class CheckBiometricAvailabilityUseCase {
  final BiometricRepository repository;

  const CheckBiometricAvailabilityUseCase(this.repository);

  Future<bool> call() => repository.canAuthenticate();
}
