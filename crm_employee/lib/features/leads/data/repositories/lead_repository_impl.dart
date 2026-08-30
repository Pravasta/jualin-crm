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

  LeadListResult _toLeadListResult(CachedResult<Map<String, dynamic>> result) {
    final data = result.data['data'] as List<dynamic>? ?? const [];
    final meta = result.data['meta'] as Map<String, dynamic>?;
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
}
