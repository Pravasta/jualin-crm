import '../../../shared/labels.dart';
import '../domain/entities/activity.dart';

/// Turns one [Activity] into what the timeline actually shows. Pure
/// function (no `BuildContext`, no Bloc) so it stays independently
/// testable — same reasoning as `lead_status.dart`. Ported from
/// `crm_dashboard/src/lib/activity-text.ts`'s `activityToTimelineEntry`,
/// with one deliberate difference: the dashboard resolves
/// `actor_membership_id` to a full name via a `namesById` map built from
/// `GET /v1/memberships` — Employee has no `ActionMembershipList`
/// (`crm_be/internal/shared/authz/authz.go`), so that lookup isn't
/// available here. Instead: "Anda" when the actor is the viewer's own
/// membership, a generic term otherwise. Confirmed against the actual
/// Claude Design mockup, which renders literal text "Ditugaskan ke Anda"
/// rather than a resolved name — the design was already built around
/// this constraint.
class TimelineEntry {
  /// true for the 3 user-authored types (note_added/call_logged/
  /// whatsapp_opened).
  final bool isHuman;
  final String text;

  /// Only set for [isHuman] entries.
  final String? authorName;

  const TimelineEntry({
    required this.isHuman,
    required this.text,
    this.authorName,
  });
}

String _actorLabel(String? actorMembershipId, String? myMembershipId) {
  if (actorMembershipId == null) return 'Seseorang';
  if (myMembershipId != null && actorMembershipId == myMembershipId) {
    return 'Anda';
  }
  // No name resolution possible (see file doc comment) — deliberately
  // vague rather than guessing a role or fabricating a name.
  return 'Anggota tim lain';
}

String? _metaStr(Map<String, dynamic>? metadata, String key) {
  final value = metadata?[key];
  return value is String && value.isNotEmpty ? value : null;
}

/// `myMembershipId` is the viewer's own `membership_id` for the current
/// organization — see `AuthBloc`'s session state.
TimelineEntry activityToTimelineEntry(
  Activity activity, {
  required String? myMembershipId,
}) {
  final meta = activity.metadata;

  switch (activity.type) {
    case 'lead_created':
      // metadata is nil for this type (crm_be's lead.Create) — the
      // source is already shown in the header, so this stays a plain,
      // static line rather than plumbing lead.source through just for
      // this one sentence.
      return const TimelineEntry(isHuman: false, text: 'Lead dibuat');

    case 'status_changed':
      final from = _metaStr(meta, 'from');
      final to = _metaStr(meta, 'to');
      final fromLabel = from != null ? statusMeta[from]?.label ?? '?' : '?';
      final toLabel = to != null ? statusMeta[to]?.label ?? '?' : '?';
      return TimelineEntry(
        isHuman: false,
        text: 'Status: $fromLabel → $toLabel',
      );

    case 'lead_assigned':
      final from = _metaStr(meta, 'from');
      final to = _metaStr(meta, 'to');
      final toName = _actorLabel(to, myMembershipId);
      return TimelineEntry(
        isHuman: false,
        text: from != null
            ? 'Dipindahkan dari ${_actorLabel(from, myMembershipId)} ke $toName'
            : 'Ditugaskan ke $toName',
      );

    case 'lead_unassigned':
      return TimelineEntry(
        isHuman: false,
        text:
            'Dilepas dari ${_actorLabel(_metaStr(meta, 'from'), myMembershipId)}',
      );

    case 'lead_converted':
      return const TimelineEntry(
        isHuman: false,
        text: 'Dikonversi menjadi customer',
      );

    case 'task_created':
      final title = _metaStr(meta, 'title');
      return TimelineEntry(
        isHuman: false,
        text: title != null ? 'Task dibuat: $title' : 'Task dibuat',
      );

    case 'task_completed':
      return const TimelineEntry(isHuman: false, text: 'Task diselesaikan');

    case 'note_added':
    case 'call_logged':
    case 'whatsapp_opened':
      return TimelineEntry(
        isHuman: true,
        text: activity.body ?? '',
        authorName: _actorLabel(activity.actorMembershipId, myMembershipId),
      );

    default:
      // Forward-compatible fallback — a future activity type this app
      // doesn't know about yet shouldn't crash the timeline, just render
      // unremarkably (mirrors the honest-degradation stance in
      // `cached_get.dart`'s doc comment).
      return TimelineEntry(isHuman: false, text: activity.type);
  }
}

String? lostReasonDisplayLabel(String? reason) {
  return reason != null ? lostReasonLabels[reason] : null;
}
