import '../../../../core/api_client.dart';

/// Talks to `/v1/leads` — list, detail, status update. Employee
/// visibility is enforced by `crm_be` itself at the repository level
/// (Phase 1 #11, confirmed directly in
/// `crm_be/internal/lead/repository_postgres.go`: `isEmployee(t)` forces
/// `assigned_to_membership_id = <caller's own membership>` regardless of
/// any `assigned_to` query param sent) — this data source never sends
/// `assigned_to` and must never be made to, relying on client-side
/// filtering as a security boundary would be exactly backwards.
abstract class LeadRemoteDataSource {
  /// Returns the full `{data, meta}` envelope (not narrowed to `data`) —
  /// `LeadRepositoryImpl` needs `meta.total`, and this is also what gets
  /// cached whole (TD §7) so a cache read back later still has it.
  Future<Map<String, dynamic>> listMyLeads({String? status, String? query});

  /// Returns the lead object itself, already narrowed to `data` — `GET
  /// /v1/leads/{id}`'s envelope has no `meta` to preserve.
  Future<Map<String, dynamic>> getLead(String id);

  /// `PATCH /v1/leads/{id}/status`. Never cached — a write. `409`
  /// (Aturan #35) surfaces as `ApiError` with `current` set; mapping
  /// that into `VersionConflictFailure` is `LeadRepositoryImpl`'s job
  /// (this layer only talks HTTP, never `Failure`).
  Future<Map<String, dynamic>> updateStatus({
    required String id,
    required int version,
    required String status,
    String? lostReason,
  });
}

class LeadRemoteDataSourceImpl implements LeadRemoteDataSource {
  final ApiClient client;

  const LeadRemoteDataSourceImpl(this.client);

  @override
  Future<Map<String, dynamic>> listMyLeads({
    String? status,
    String? query,
  }) {
    return client.sendListEnvelope(leadsListPath(status: status, query: query));
  }

  @override
  Future<Map<String, dynamic>> getLead(String id) async {
    final data = await client.send('/v1/leads/$id');
    return data as Map<String, dynamic>;
  }

  @override
  Future<Map<String, dynamic>> updateStatus({
    required String id,
    required int version,
    required String status,
    String? lostReason,
  }) async {
    final data = await client.send(
      '/v1/leads/$id/status',
      method: 'PATCH',
      body: {
        'version': version,
        'status': status,
        'lost_reason': ?lostReason,
      },
    );
    return data as Map<String, dynamic>;
  }
}

/// Pure — also what `LeadRepositoryImpl` uses as the cache key (TD §7's
/// own example key shape: `"GET /v1/leads?status=new&page=1"`), so the
/// same request always resolves to the same cache row.
String leadsListPath({String? status, String? query}) {
  final params = <String, String>{};
  // `null`/empty means "no filter" — the presentation layer's "Semua"
  // chip is responsible for sending null, not a magic string that would
  // need handling at every layer between here and there.
  if (status != null && status.isNotEmpty) {
    params['status'] = status;
  }
  if (query != null && query.trim().isNotEmpty) {
    params['q'] = query.trim();
  }
  if (params.isEmpty) return '/v1/leads';
  return '/v1/leads?${Uri(queryParameters: params).query}';
}
