import 'package:equatable/equatable.dart';

/// Shape read from `crm_be/internal/notification/handler_http.go`'s
/// `notificationJSON`. `title`/`body` are already server-rendered
/// Indonesian text (unlike `Activity`, there's no per-type formatting
/// this app needs to do — the record itself carries what to show).
///
/// `leadId` is the only deeplink target this screen can actually reach:
/// `type` is `ck_notifications_type`'s `'lead_assigned'` or
/// `'task_assigned'`, but only `lead_assigned` is ever created
/// (`internal/lead/usecase.go`'s `pushAssignmentNotification` —
/// confirmed by search, nothing in `internal/task` creates a
/// `task_assigned` row despite the CHECK constraint allowing it).
class NotificationItem extends Equatable {
  final String id;
  final String type;
  final String? leadId;
  final String? taskId;
  final String title;
  final String body;
  final DateTime? readAt;
  final DateTime createdAt;

  const NotificationItem({
    required this.id,
    required this.type,
    this.leadId,
    this.taskId,
    required this.title,
    required this.body,
    this.readAt,
    required this.createdAt,
  });

  bool get isUnread => readAt == null;

  @override
  List<Object?> get props => [
    id,
    type,
    leadId,
    taskId,
    title,
    body,
    readAt,
    createdAt,
  ];
}

/// `GET /v1/notifications` — deliberately WITHOUT `fromCache`/`fetchedAt`
/// fields `LeadListResult`/`TaskListResult` both carry: TD §7 names only
/// four endpoints as cacheable (`/v1/leads`, `/v1/tasks`, `/v1/leads/
/// {id}`, `/v1/leads/{id}/activities`) — `/v1/notifications` isn't one
/// of them. A small gap between design brief §10's cache-banner table
/// (silent on Notifikasi either way) and TD §7's explicit list — noted
/// in `docs/issues/073-*.md`, not silently resolved by guessing.
class NotificationListResult extends Equatable {
  final List<NotificationItem> notifications;

  const NotificationListResult({required this.notifications});

  @override
  List<Object?> get props => [notifications];
}
