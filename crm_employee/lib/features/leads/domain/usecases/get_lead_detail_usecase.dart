import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/lead.dart';
import '../repositories/lead_repository.dart';

class GetLeadDetailUseCase implements UseCase<LeadDetailResult, String> {
  final LeadRepository repository;

  const GetLeadDetailUseCase(this.repository);

  @override
  Future<Either<Failure, LeadDetailResult>> call(String leadId) {
    return repository.getLeadDetail(leadId);
  }
}
