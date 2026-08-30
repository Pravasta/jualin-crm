import 'package:dartz/dartz.dart' hide Task;
import 'package:equatable/equatable.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/task.dart';
import '../repositories/task_repository.dart';

class CompleteTaskParams extends Equatable {
  final String id;
  final int version;

  const CompleteTaskParams({required this.id, required this.version});

  @override
  List<Object?> get props => [id, version];
}

/// One-way (design brief §7.4) — no `UncompleteTaskUseCase` exists, on
/// purpose. A stale [CompleteTaskParams.version] surfaces as
/// `Left(VersionConflictFailure<Task>)` (Aturan #35).
class CompleteTaskUseCase implements UseCase<Task, CompleteTaskParams> {
  final TaskRepository repository;

  const CompleteTaskUseCase(this.repository);

  @override
  Future<Either<Failure, Task>> call(CompleteTaskParams params) {
    return repository.completeTask(id: params.id, version: params.version);
  }
}
