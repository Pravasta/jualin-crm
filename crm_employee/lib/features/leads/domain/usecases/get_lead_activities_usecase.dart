import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/activity.dart';
import '../repositories/activity_repository.dart';

class GetLeadActivitiesUseCase implements UseCase<ActivityListResult, String> {
  final ActivityRepository repository;

  const GetLeadActivitiesUseCase(this.repository);

  @override
  Future<Either<Failure, ActivityListResult>> call(String leadId) {
    return repository.getActivities(leadId);
  }
}
