import '../../../../core/api_client.dart';

/// Talks to `/v1/leads/{id}/activities` — nothing else. `crm_be` never
/// exposes `PATCH`/`DELETE` on this resource at all (append-only, TD
/// phase 2 §1.3) — there is no method here to remove or edit an
/// activity, on purpose.
abstract class ActivityRemoteDataSource {
  /// Returns the raw list, already narrowed to `data` — this envelope
  /// has no `meta` (Aturan #33's plain-list form, not the paginated
  /// one).
  Future<List<dynamic>> listActivities(String leadId);

  /// Only [userActivityTypes] (`activity.dart`) are accepted by
  /// `crm_be` — anything else is `422`. Never cached — a write.
  Future<Map<String, dynamic>> createActivity({
    required String leadId,
    required String type,
    String? body,
  });
}

class ActivityRemoteDataSourceImpl implements ActivityRemoteDataSource {
  final ApiClient client;

  const ActivityRemoteDataSourceImpl(this.client);

  @override
  Future<List<dynamic>> listActivities(String leadId) async {
    final data = await client.send('/v1/leads/$leadId/activities');
    return data as List<dynamic>;
  }

  @override
  Future<Map<String, dynamic>> createActivity({
    required String leadId,
    required String type,
    String? body,
  }) async {
    final data = await client.send(
      '/v1/leads/$leadId/activities',
      method: 'POST',
      body: {'type': type, 'body': ?body},
    );
    return data as Map<String, dynamic>;
  }
}

/// The cache key for [listActivities] — `LeadRepositoryImpl` and
/// `ActivityRepositoryImpl` both need this (the former to invalidate on
/// a successful status/note write... except TD §7 doesn't ask for
/// invalidation, only "last successful response"; kept here anyway so
/// the exact key string is defined in exactly one place).
String activitiesListPath(String leadId) => '/v1/leads/$leadId/activities';
