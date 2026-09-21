import 'package:equatable/equatable.dart';

/// Shape read from `crm_be/internal/task/handler_http.go`'s `taskJSON` —
/// only the fields My Tasks (#73) actually uses. `assigned_to_
/// membership_id`/`completed_by_membership_id`/`created_by_membership_id`
/// aren't rendered anywhere on this screen, so they're left out — same
/// "only what's used" discipline `Lead`'s own doc comment follows.
class Task extends Equatable {
  final String id;
  final String leadId;
  final String title;
  final String? description;
  final DateTime? dueAt;

  /// `ck_tasks_status`: `'open'` or `'done'` — TD phase 2 §5. Completion
  /// is one-way (design brief §7.4: "menandai selesai satu arah — tidak
  /// bisa dibuka kembali"); nothing in this app ever sends `'open'`.
  final String status;
  final int version;

  /// Set once the task is `'done'` — what the Selesai tab (issue #148)
  /// shows and orders by. `crm_be` has always sent it; this screen simply
  /// never showed a completed task before.
  final DateTime? completedAt;

  const Task({
    required this.id,
    required this.leadId,
    required this.title,
    this.description,
    this.dueAt,
    required this.status,
    required this.version,
    this.completedAt,
  });

  @override
  List<Object?> get props => [
    id,
    leadId,
    title,
    description,
    dueAt,
    status,
    version,
    completedAt,
  ];
}

/// `GET /v1/tasks` — plus whether it came from the offline cache (TD §7
/// names `/v1/tasks` as one of the four cacheable endpoints, the same
/// treatment `LeadListResult` gets for `/v1/leads`).
class TaskListResult extends Equatable {
  final List<Task> tasks;
  final bool fromCache;
  final DateTime? fetchedAt;

  const TaskListResult({
    required this.tasks,
    required this.fromCache,
    this.fetchedAt,
  });

  @override
  List<Object?> get props => [tasks, fromCache, fetchedAt];
}
