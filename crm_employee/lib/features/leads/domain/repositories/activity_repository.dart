import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../entities/activity.dart';

abstract class ActivityRepository {
  Future<Either<Failure, ActivityListResult>> getActivities(String leadId);

  /// `type` must be one of [userActivityTypes] — the caller (a use case)
  /// is what guarantees that, not this interface.
  Future<Either<Failure, Activity>> createActivity({
    required String leadId,
    required String type,
    String? body,
  });
}
