import '../repositories/external_action_repository.dart';

/// Same plain-boolean shape as `LaunchDialerUseCase` — the repository
/// already collapses every failure into `false`.
class LaunchEmailUseCase {
  final ExternalActionRepository repository;

  const LaunchEmailUseCase(this.repository);

  Future<bool> call(String email) => repository.launchEmail(email);
}
