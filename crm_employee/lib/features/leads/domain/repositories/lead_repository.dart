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
}
