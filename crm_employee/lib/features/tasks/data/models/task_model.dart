import '../../domain/entities/task.dart';

class TaskModel extends Task {
  const TaskModel({
    required super.id,
    required super.leadId,
    required super.title,
    super.description,
    super.dueAt,
    required super.status,
    required super.version,
  });

  factory TaskModel.fromJson(Map<String, dynamic> json) {
    return TaskModel(
      id: json['id'] as String,
      leadId: json['lead_id'] as String,
      title: json['title'] as String,
      description: json['description'] as String?,
      dueAt: json['due_at'] != null
          ? DateTime.parse(json['due_at'] as String)
          : null,
      status: json['status'] as String,
      version: json['version'] as int,
    );
  }
}
