import 'package:dartz/dartz.dart';
import 'package:equatable/equatable.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/activity.dart';
import '../repositories/activity_repository.dart';

class LogCallParams extends Equatable {
  final String leadId;
  final String phone;

  const LogCallParams({required this.leadId, required this.phone});

  @override
  List<Object?> get props => [leadId, phone];
}

/// Called by `LeadDetailBloc` only AFTER `ExternalActionRepository.
/// launchDialer` reports the dialer actually opened (design brief §8.3)
/// — never at button-press time.
class LogCallUseCase implements UseCase<Activity, LogCallParams> {
  final ActivityRepository repository;

  const LogCallUseCase(this.repository);

  @override
  Future<Either<Failure, Activity>> call(LogCallParams params) {
    return repository.createActivity(
      leadId: params.leadId,
      type: 'call_logged',
      body: 'Menelepon ${params.phone}',
    );
  }
}
