import 'package:equatable/equatable.dart';

/// Shape read from `crm_be/internal/activity/{handler_http,entity}.go`.
/// `metadata` is left as a raw, undecoded map — its shape differs per
/// `type` (`crm_be`'s own doc comment: "tidak pernah di-query, hanya
/// dirender"), so `activity_text.dart` is what interprets it per type,
/// not this entity.
class Activity extends Equatable {
  final String id;
  final String leadId;
  final String type;
  final String? actorMembershipId;
  final String? body;
  final Map<String, dynamic>? metadata;
  final DateTime createdAt;

  const Activity({
    required this.id,
    required this.leadId,
    required this.type,
    this.actorMembershipId,
    this.body,
    this.metadata,
    required this.createdAt,
  });

  @override
  List<Object?> get props => [
    id,
    leadId,
    type,
    actorMembershipId,
    body,
    metadata,
    createdAt,
  ];
}

/// The three types `POST /v1/leads/{id}/activities` accepts from a
/// client (`crm_be/internal/activity/entity.go`'s `userTypes` — every
/// other value in `ck_activities_type` is system-generated and gets 422
/// if a client tries to submit it).
const List<String> userActivityTypes = [
  'note_added',
  'call_logged',
  'whatsapp_opened',
];

/// `GET /v1/leads/{id}/activities` — plus whether it came from the
/// offline cache and, if so, when it was fetched (TD §7).
class ActivityListResult extends Equatable {
  final List<Activity> activities;
  final bool fromCache;
  final DateTime? fetchedAt;

  const ActivityListResult({
    required this.activities,
    required this.fromCache,
    this.fetchedAt,
  });

  @override
  List<Object?> get props => [activities, fromCache, fetchedAt];
}
