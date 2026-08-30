import 'package:dartz/dartz.dart';
import 'package:equatable/equatable.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/lead.dart';
import '../repositories/lead_repository.dart';

class UpdateLeadStatusParams extends Equatable {
  final String leadId;
  final int version;
  final String status;
  final String? lostReason;

  const UpdateLeadStatusParams({
    required this.leadId,
    required this.version,
    required this.status,
    this.lostReason,
  });

  @override
  List<Object?> get props => [leadId, version, status, lostReason];
}

/// Whether the UI is even allowed to offer `params.status` from
/// `params.leadId`'s current status is `lead_status.dart`'s job, checked
/// BEFORE this use case is ever called — this class does not re-validate
/// the transition, only sends it. A stale `version` (someone else moved
/// the lead first) surfaces as `Left(VersionConflictFailure<Lead>)`
/// (Aturan #35) for the bloc to turn into the "muat ulang" dialog.
class UpdateLeadStatusUseCase
    implements UseCase<Lead, UpdateLeadStatusParams> {
  final LeadRepository repository;

  const UpdateLeadStatusUseCase(this.repository);

  @override
  Future<Either<Failure, Lead>> call(UpdateLeadStatusParams params) {
    return repository.updateStatus(
      id: params.leadId,
      version: params.version,
      status: params.status,
      lostReason: params.lostReason,
    );
  }
}
