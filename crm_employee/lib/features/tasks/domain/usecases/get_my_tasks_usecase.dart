import 'package:dartz/dartz.dart' hide Task;
import 'package:equatable/equatable.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/task.dart';
import '../repositories/task_repository.dart';

class GetMyTasksParams extends Equatable {
  final String assignedTo;
  final String? status;

  const GetMyTasksParams({required this.assignedTo, this.status});

  @override
  List<Object?> get props => [assignedTo, status];
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
    );
  }
}
