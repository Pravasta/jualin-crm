import 'package:dartz/dartz.dart';
import 'package:equatable/equatable.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/activity.dart';
import '../repositories/activity_repository.dart';

class AddLeadNoteParams extends Equatable {
  final String leadId;
  final String body;

  const AddLeadNoteParams({required this.leadId, required this.body});

  @override
  List<Object?> get props => [leadId, body];
}

/// Non-empty `body` (design brief §10's "kesalahan validasi per field",
/// the note form's own state) is checked in the bloc, before dispatch —
/// this use case never receives an empty string to reject a second time.
class AddLeadNoteUseCase implements UseCase<Activity, AddLeadNoteParams> {
  final ActivityRepository repository;

  const AddLeadNoteUseCase(this.repository);

  @override
  Future<Either<Failure, Activity>> call(AddLeadNoteParams params) {
    return repository.createActivity(
      leadId: params.leadId,
      type: 'note_added',
      body: params.body,
    );
  }
}
