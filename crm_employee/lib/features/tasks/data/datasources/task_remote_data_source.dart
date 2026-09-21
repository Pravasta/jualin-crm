import '../../../../core/api_client.dart';

abstract class TaskRemoteDataSource {
  /// Returns the full `{data, meta}` envelope (not narrowed to `data`)
  /// — cached whole (TD §7), same reasoning `LeadRemoteDataSource.
  /// listMyLeads` returns it whole.
  Future<Map<String, dynamic>> listMyTasks({
    required String assignedTo,
    String? status,
    int? perPage,
  });

  /// `POST /v1/tasks/{id}/complete`. Never cached — a write. `409`
  /// surfaces as `ApiError` with `current` set; mapping that into
  /// `VersionConflictFailure` is `TaskRepositoryImpl`'s job.
  Future<Map<String, dynamic>> complete({
    required String id,
    required int version,
  });
}

class TaskRemoteDataSourceImpl implements TaskRemoteDataSource {
  final ApiClient client;

  const TaskRemoteDataSourceImpl(this.client);

  @override
  Future<Map<String, dynamic>> listMyTasks({
    required String assignedTo,
    String? status,
    int? perPage,
  }) {
    return client.sendListEnvelope(
      tasksListPath(assignedTo: assignedTo, status: status, perPage: perPage),
    );
  }

  @override
  Future<Map<String, dynamic>> complete({
    required String id,
    required int version,
  }) async {
    final data = await client.send(
      '/v1/tasks/$id/complete',
      method: 'POST',
      body: {'version': version},
    );
    return data as Map<String, dynamic>;
  }
}

/// Pure — also what `TaskRepositoryImpl` uses as the cache key, same
/// pattern `leadsListPath` follows.
String tasksListPath({required String assignedTo, String? status, int? perPage}) {
  final params = <String, String>{'assigned_to': assignedTo};
  if (status != null && status.isNotEmpty) {
    params['status'] = status;
  }
  if (perPage != null) {
    params['per_page'] = '$perPage';
  }
  return '/v1/tasks?${Uri(queryParameters: params).query}';
}
