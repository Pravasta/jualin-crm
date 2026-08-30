import '../../domain/entities/activity.dart';

class ActivityModel extends Activity {
  const ActivityModel({
    required super.id,
    required super.leadId,
    required super.type,
    super.actorMembershipId,
    super.body,
    super.metadata,
    required super.createdAt,
  });

  factory ActivityModel.fromJson(Map<String, dynamic> json) {
    return ActivityModel(
      id: json['id'] as String,
      leadId: json['lead_id'] as String,
      type: json['type'] as String,
      actorMembershipId: json['actor_membership_id'] as String?,
      body: json['body'] as String?,
      metadata: json['metadata'] as Map<String, dynamic>?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
