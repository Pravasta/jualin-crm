import '../../domain/entities/lead.dart';

class LeadModel extends Lead {
  const LeadModel({
    required super.id,
    required super.leadNumber,
    required super.name,
    super.email,
    super.phone,
    super.phoneE164,
    super.company,
    super.notes,
    required super.status,
    super.lostReason,
    required super.source,
    super.assignedToMembershipId,
    required super.version,
    required super.createdAt,
    required super.updatedAt,
  });

  factory LeadModel.fromJson(Map<String, dynamic> json) {
    return LeadModel(
      id: json['id'] as String,
      leadNumber: json['lead_number'] as int,
      name: json['name'] as String,
      email: json['email'] as String?,
      phone: json['phone'] as String?,
      phoneE164: json['phone_e164'] as String?,
      company: json['company'] as String?,
      notes: json['notes'] as String?,
      status: json['status'] as String,
      lostReason: json['lost_reason'] as String?,
      source: json['source'] as String,
      assignedToMembershipId: json['assigned_to_membership_id'] as String?,
      version: json['version'] as int,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }
}
