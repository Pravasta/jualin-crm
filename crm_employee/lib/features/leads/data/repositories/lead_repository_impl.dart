import 'package:dartz/dartz.dart';

import '../../../../core/cache/cached_get.dart';
import '../../../../core/cache/response_cache.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/network/run_api_call.dart';
import '../../domain/entities/lead.dart';
import '../../domain/repositories/lead_repository.dart';
import '../datasources/lead_remote_data_source.dart';
import '../models/lead_model.dart';

class LeadRepositoryImpl implements LeadRepository {
  final LeadRemoteDataSource remoteDataSource;
  final ResponseCache responseCache;

  const LeadRepositoryImpl({
    required this.remoteDataSource,
    required this.responseCache,
  });

  @override
  Future<Either<Failure, LeadListResult>> getMyLeads({
    String? status,
    String? query,
  }) {
    final path = leadsListPath(status: status, query: query);
    final cacheKey = 'GET $path';

    return runApiCall(() async {
      final result = await cachedGet(
        cache: responseCache,
        key: cacheKey,
        fetch: () => remoteDataSource.listMyLeads(status: status, query: query),
      );
      return _toLeadListResult(result);
    });
  }

  LeadListResult _toLeadListResult(CachedResult<dynamic> result) {
    final envelope = result.data as Map<String, dynamic>;
    final data = envelope['data'] as List<dynamic>? ?? const [];
    final meta = envelope['meta'] as Map<String, dynamic>?;
    final total = meta?['total'] as int? ?? data.length;
    final leads = data
        .map((json) => LeadModel.fromJson(json as Map<String, dynamic>))
        .toList();

    return LeadListResult(
      leads: leads,
      total: total,
      fromCache: result.fromCache,
      fetchedAt: result.fetchedAt,
    );
  }

  @override
  Future<Either<Failure, LeadDetailResult>> getLeadDetail(String id) {
    final cacheKey = 'GET /v1/leads/$id';

    return runApiCall(() async {
      final result = await cachedGet(
        cache: responseCache,
        key: cacheKey,
        fetch: () => remoteDataSource.getLead(id),
      );
      return LeadDetailResult(
        lead: LeadModel.fromJson(result.data as Map<String, dynamic>),
        fromCache: result.fromCache,
        fetchedAt: result.fetchedAt,
      );
    });
  }

  @override
  Future<Either<Failure, Lead>> updateStatus({
    required String id,
    required int version,
    required String status,
    String? lostReason,
  }) {
    return runApiCall(
      () => remoteDataSource
          .updateStatus(
            id: id,
            version: version,
            status: status,
            lostReason: lostReason,
          )
          .then(LeadModel.fromJson),
      onApiError: (e) => e.code == 'version_conflict' && e.current != null
          ? VersionConflictFailure<Lead>(
              e.message,
              LeadModel.fromJson(e.current!),
            )
          : UnexpectedFailure(e.message),
    );
  }
}
