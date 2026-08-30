import '../../domain/entities/notification.dart';

class NotificationModel extends NotificationItem {
  const NotificationModel({
    required super.id,
    required super.type,
    super.leadId,
    super.taskId,
    required super.title,
    required super.body,
    super.readAt,
    required super.createdAt,
  });

  factory NotificationModel.fromJson(Map<String, dynamic> json) {
    return NotificationModel(
      id: json['id'] as String,
      type: json['type'] as String,
      leadId: json['lead_id'] as String?,
      taskId: json['task_id'] as String?,
      title: json['title'] as String,
      body: json['body'] as String,
      readAt: json['read_at'] != null
          ? DateTime.parse(json['read_at'] as String)
          : null,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
