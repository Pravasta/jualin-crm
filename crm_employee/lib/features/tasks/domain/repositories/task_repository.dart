// dartz exports its own `Task` (a lazy-computation type this app never
// uses) — hidden so it doesn't collide with the domain entity below.
import 'package:dartz/dartz.dart' hide Task;

import '../../../../core/error/failures.dart';
import '../entities/task.dart';

abstract class TaskRepository {
  /// `assignedTo` is always the caller's own `membership_id` in
  /// practice ("Tugas Saya" — literally MY tasks). Unlike `leads`, the
  /// backend does NOT automatically scope an Employee's `GET /v1/tasks`
  /// to tasks assigned to them — `internal/task/repository_postgres.go`'s
  /// `isEmployee(t)` branch only restricts to tasks whose LEAD is
  /// assigned to them, a broader set. Sending `assigned_to` explicitly
  /// is what actually narrows it to "my tasks" — confirmed directly in
  /// `buildTaskWhere`, not assumed from the endpoint's name.
  Future<Either<Failure, TaskListResult>> getMyTasks({
    required String assignedTo,
    String? status,
  });

  /// `POST /v1/tasks/{id}/complete`. One-way (design brief §7.4) — there
  /// is no `uncomplete`, on purpose, not an oversight. A stale [version]
  /// surfaces as `VersionConflictFailure<Task>` (Aturan #35).
  Future<Either<Failure, Task>> completeTask({
    required String id,
    required int version,
  });
}
