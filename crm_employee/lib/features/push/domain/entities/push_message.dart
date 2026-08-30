import 'package:equatable/equatable.dart';

/// One FCM message, translated out of `firebase_messaging`'s
/// `RemoteMessage` — the domain layer has no business knowing that
/// package exists, only `FirebaseMessagingDataSource`/`PushRepositoryImpl`
/// (data layer) do.
///
/// `leadId` is `crm_be`'s `data.lead_id` (`internal/lead/usecase.go`'s
/// `pushAssignmentNotification`, confirmed directly in source) — the
/// only key this app currently reads. Only `type: lead_assigned` is
/// ever actually sent (`internal/notification`'s `ck_notifications_type`
/// also allows `task_assigned`, but nothing in `internal/task` creates
/// one — confirmed by search, not assumed), so `leadId` is the only
/// deeplink target that exists in practice.
class PushMessage extends Equatable {
  final String? leadId;
  final String? title;
  final String? body;

  const PushMessage({this.leadId, this.title, this.body});

  @override
  List<Object?> get props => [leadId, title, body];
}
