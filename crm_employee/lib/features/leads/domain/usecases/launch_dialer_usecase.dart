import '../repositories/external_action_repository.dart';

/// Same reasoning as auth's `CheckBiometricAvailabilityUseCase` — a
/// plain boolean outcome with no meaningful `Failure` to represent
/// (`ExternalActionRepository` already collapses every failure mode into
/// `false`), not `Either`-wrapped.
class LaunchDialerUseCase {
  final ExternalActionRepository repository;

  const LaunchDialerUseCase(this.repository);

  Future<bool> call(String phone) => repository.launchDialer(phone);
}
