import 'package:dartz/dartz.dart' hide Task;
import 'package:equatable/equatable.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/task.dart';
import '../repositories/task_repository.dart';

class GetMyTasksParams extends Equatable {
  final String assignedTo;
  final String? status;

  /// Null = crm_be's default page (25). The Selesai tab asks for more,
  /// since history is the list that keeps growing.
  final int? perPage;

  const GetMyTasksParams({required this.assignedTo, this.status, this.perPage});

  @override
  List<Object?> get props => [assignedTo, status, perPage];
}

class GetMyTasksUseCase
    implements UseCase<TaskListResult, GetMyTasksParams> {
  final TaskRepository repository;

  const GetMyTasksUseCase(this.repository);

  @override
  Future<Either<Failure, TaskListResult>> call(GetMyTasksParams params) {
    return repository.getMyTasks(
      assignedTo: params.assignedTo,
      status: params.status,
      perPage: params.perPage,
    );
  }
}
