import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/activity.dart';
import '../repositories/activity_repository.dart';

/// Called by `LeadDetailBloc` only AFTER `ExternalActionRepository.
/// launchWhatsApp` reports WhatsApp actually opened (design brief §8.3)
/// — never at button-press time. `String` param is the lead id; unlike
/// [LogCallUseCase] the body text doesn't need the phone number (the
/// activity type itself already says "WhatsApp", and the lead's own
/// detail screen already shows the number).
class LogWhatsAppOpenedUseCase implements UseCase<Activity, String> {
  final ActivityRepository repository;

  const LogWhatsAppOpenedUseCase(this.repository);

  @override
  Future<Either<Failure, Activity>> call(String leadId) {
    return repository.createActivity(
      leadId: leadId,
      type: 'whatsapp_opened',
      body: 'Membuka WhatsApp',
    );
  }
}
