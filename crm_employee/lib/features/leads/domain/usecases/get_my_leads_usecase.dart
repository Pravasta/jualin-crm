import 'package:dartz/dartz.dart';
import 'package:equatable/equatable.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/lead.dart';
import '../repositories/lead_repository.dart';

class GetMyLeadsParams extends Equatable {
  final String? status;
  final String? query;

  const GetMyLeadsParams({this.status, this.query});

  @override
  List<Object?> get props => [status, query];
}

class GetMyLeadsUseCase implements UseCase<LeadListResult, GetMyLeadsParams> {
  final LeadRepository repository;

  const GetMyLeadsUseCase(this.repository);

  @override
  Future<Either<Failure, LeadListResult>> call(GetMyLeadsParams params) {
    return repository.getMyLeads(status: params.status, query: params.query);
  }
}
