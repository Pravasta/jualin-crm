// dartz exports its own `Task` — hidden, same reasoning as
// task_repository.dart.
import 'package:dartz/dartz.dart' hide Task;

import '../../../../core/cache/cached_get.dart';
import '../../../../core/cache/response_cache.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/network/run_api_call.dart';
import '../../domain/entities/task.dart';
import '../../domain/repositories/task_repository.dart';
import '../datasources/task_remote_data_source.dart';
import '../models/task_model.dart';

class TaskRepositoryImpl implements TaskRepository {
  final TaskRemoteDataSource remoteDataSource;
  final ResponseCache responseCache;

  const TaskRepositoryImpl({
    required this.remoteDataSource,
    required this.responseCache,
  });

  @override
  Future<Either<Failure, TaskListResult>> getMyTasks({
    required String assignedTo,
    String? status,
  }) {
    final path = tasksListPath(assignedTo: assignedTo, status: status);
    final cacheKey = 'GET $path';

    return runApiCall(() async {
      final result = await cachedGet(
        cache: responseCache,
        key: cacheKey,
        fetch: () => remoteDataSource.listMyTasks(
          assignedTo: assignedTo,
          status: status,
        ),
      );
      final envelope = result.data as Map<String, dynamic>;
      final data = envelope['data'] as List<dynamic>? ?? const [];
      final tasks = data
          .map((json) => TaskModel.fromJson(json as Map<String, dynamic>))
          .toList();

      return TaskListResult(
        tasks: tasks,
        fromCache: result.fromCache,
        fetchedAt: result.fetchedAt,
      );
    });
  }

  @override
  Future<Either<Failure, Task>> completeTask({
    required String id,
    required int version,
  }) {
    return runApiCall(
      () => remoteDataSource
          .complete(id: id, version: version)
          .then(TaskModel.fromJson),
      onApiError: (e) => e.code == 'version_conflict' && e.current != null
          ? VersionConflictFailure<Task>(e.message, TaskModel.fromJson(e.current!))
          : UnexpectedFailure(e.message),
    );
  }
}
