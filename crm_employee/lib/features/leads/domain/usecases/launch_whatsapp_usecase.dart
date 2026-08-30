import '../repositories/external_action_repository.dart';

/// Same reasoning as [LaunchDialerUseCase] — a plain boolean outcome,
/// not `Either`-wrapped. Callers must pass `Lead.phoneE164`, never
/// `Lead.phone` — see `ExternalActionRepository.launchWhatsApp`'s doc
/// comment for why.
class LaunchWhatsAppUseCase {
  final ExternalActionRepository repository;

  const LaunchWhatsAppUseCase(this.repository);

  Future<bool> call(String phoneE164) => repository.launchWhatsApp(phoneE164);
}
