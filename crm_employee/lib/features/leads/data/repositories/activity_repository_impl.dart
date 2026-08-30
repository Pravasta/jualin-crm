import 'package:dartz/dartz.dart';

import '../../../../core/cache/cached_get.dart';
import '../../../../core/cache/response_cache.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/network/run_api_call.dart';
import '../../domain/entities/activity.dart';
import '../../domain/repositories/activity_repository.dart';
import '../datasources/activity_remote_data_source.dart';
import '../models/activity_model.dart';

class ActivityRepositoryImpl implements ActivityRepository {
  final ActivityRemoteDataSource remoteDataSource;
  final ResponseCache responseCache;

  const ActivityRepositoryImpl({
    required this.remoteDataSource,
    required this.responseCache,
  });

  @override
  Future<Either<Failure, ActivityListResult>> getActivities(String leadId) {
    final cacheKey = 'GET ${activitiesListPath(leadId)}';

    return runApiCall(() async {
      final result = await cachedGet(
        cache: responseCache,
        key: cacheKey,
        fetch: () => remoteDataSource.listActivities(leadId),
      );
      final list = (result.data as List<dynamic>)
          .map((json) => ActivityModel.fromJson(json as Map<String, dynamic>))
          .toList();
      return ActivityListResult(
        activities: list,
        fromCache: result.fromCache,
        fetchedAt: result.fetchedAt,
      );
    });
  }

  @override
  Future<Either<Failure, Activity>> createActivity({
    required String leadId,
    required String type,
    String? body,
  }) {
    return runApiCall(() async {
      final json = await remoteDataSource.createActivity(
        leadId: leadId,
        type: type,
        body: body,
      );
      return ActivityModel.fromJson(json);
    });
  }
}
