import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../entities/lead.dart';

abstract class LeadRepository {
  /// `status` is a single backend status value, or `null` for no filter
  /// ("Semua") — the horizontal chip row (design brief §6) is single-select,
  /// not the multi-select the design's own "Filter status" bottom sheet
  /// mockup shows (a genuine inconsistency between the two states the
  /// design produced — see `docs/issues/071-my-leads.md`). `query` is
  /// the free-text search box; issue #71's own cakupan asks for it even
  /// though no mockup state shows one.
  Future<Either<Failure, LeadListResult>> getMyLeads({
    String? status,
    String? query,
  });

  /// `GET /v1/leads/{id}` — Employee visibility is enforced by `crm_be`
  /// the same way as [getMyLeads]; a lead not assigned to this employee
  /// 404s (Aturan #6), never 403.
  Future<Either<Failure, LeadDetailResult>> getLeadDetail(String id);

  /// `PATCH /v1/leads/{id}/status`. `lostReason` is required by
  /// `crm_be` only when `status == 'lost'` — enforced by
  /// `lead_status.dart`'s pure logic before this is ever called, not
  /// re-validated here. A stale [version] surfaces as
  /// `VersionConflictFailure<Lead>` (Aturan #35), never silently retried
  /// or overwritten.
  Future<Either<Failure, Lead>> updateStatus({
    required String id,
    required int version,
    required String status,
    String? lostReason,
  });
}
