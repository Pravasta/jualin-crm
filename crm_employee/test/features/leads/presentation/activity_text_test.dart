import 'package:crm_employee/features/leads/domain/entities/activity.dart';
import 'package:crm_employee/features/leads/presentation/activity_text.dart';
import 'package:flutter_test/flutter_test.dart';

Activity _activity({
  String type = 'note_added',
  String? actorMembershipId,
  String? body,
  Map<String, dynamic>? metadata,
}) {
  return Activity(
    id: 'activity-1',
    leadId: 'lead-1',
    type: type,
    actorMembershipId: actorMembershipId,
    body: body,
    metadata: metadata,
    createdAt: DateTime.utc(2026, 8, 30),
  );
}

const _me = 'membership-me';
const _other = 'membership-other';

void main() {
  group('activityToTimelineEntry', () {
    test('lead_created renders a static line, ignores metadata', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'lead_created'),
        myMembershipId: _me,
      );
      expect(entry.isHuman, isFalse);
      expect(entry.text, 'Lead dibuat');
    });

    test('status_changed renders both labels from metadata', () {
      final entry = activityToTimelineEntry(
        _activity(
          type: 'status_changed',
          metadata: {'from': 'new', 'to': 'contacted'},
        ),
        myMembershipId: _me,
      );
      expect(entry.isHuman, isFalse);
      expect(entry.text, 'Status: Baru → Dihubungi');
    });

    test('status_changed falls back to "?" when metadata is missing a key', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'status_changed', metadata: {'from': 'new'}),
        myMembershipId: _me,
      );
      expect(entry.text, 'Status: Baru → ?');
    });

    test('lead_assigned to the viewer renders "Ditugaskan ke Anda"', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'lead_assigned', metadata: {'to': _me}),
        myMembershipId: _me,
      );
      expect(entry.text, 'Ditugaskan ke Anda');
    });

    test('lead_assigned to someone else never claims a resolved name', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'lead_assigned', metadata: {'to': _other}),
        myMembershipId: _me,
      );
      expect(entry.text, 'Ditugaskan ke Anggota tim lain');
    });

    test('lead_assigned with both from/to renders the move', () {
      final entry = activityToTimelineEntry(
        _activity(
          type: 'lead_assigned',
          metadata: {'from': _other, 'to': _me},
        ),
        myMembershipId: _me,
      );
      expect(entry.text, 'Dipindahkan dari Anggota tim lain ke Anda');
    });

    test('lead_unassigned', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'lead_unassigned', metadata: {'from': _me}),
        myMembershipId: _me,
      );
      expect(entry.text, 'Dilepas dari Anda');
    });

    test('lead_converted renders a static line', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'lead_converted'),
        myMembershipId: _me,
      );
      expect(entry.text, 'Dikonversi menjadi customer');
    });

    test('task_created includes the title when present', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'task_created', metadata: {'title': 'Follow up'}),
        myMembershipId: _me,
      );
      expect(entry.text, 'Task dibuat: Follow up');
    });

    test('task_created without a title falls back to a plain line', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'task_created'),
        myMembershipId: _me,
      );
      expect(entry.text, 'Task dibuat');
    });

    test('task_completed renders a static line', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'task_completed'),
        myMembershipId: _me,
      );
      expect(entry.text, 'Task diselesaikan');
    });

    for (final type in ['note_added', 'call_logged', 'whatsapp_opened']) {
      test('$type is human-authored, renders body verbatim, author "Anda" for self', () {
        final entry = activityToTimelineEntry(
          _activity(type: type, actorMembershipId: _me, body: 'halo'),
          myMembershipId: _me,
        );
        expect(entry.isHuman, isTrue);
        expect(entry.text, 'halo');
        expect(entry.authorName, 'Anda');
      });

      test('$type author is generic, never a resolved name, for another actor', () {
        final entry = activityToTimelineEntry(
          _activity(type: type, actorMembershipId: _other, body: 'halo'),
          myMembershipId: _me,
        );
        expect(entry.authorName, 'Anggota tim lain');
      });

      test('$type falls back to empty body, never null/"null" text', () {
        final entry = activityToTimelineEntry(
          _activity(type: type, actorMembershipId: _me),
          myMembershipId: _me,
        );
        expect(entry.text, '');
      });
    }

    test('an unrecognized type never throws — renders the raw type instead', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'something_new'),
        myMembershipId: _me,
      );
      expect(entry.isHuman, isFalse);
      expect(entry.text, 'something_new');
    });

    test('null actor renders "Seseorang", not "Anda"', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'note_added', body: 'x'),
        myMembershipId: _me,
      );
      expect(entry.authorName, 'Seseorang');
    });

    test('myMembershipId null never matches — everyone renders as generic, never "Anda"', () {
      final entry = activityToTimelineEntry(
        _activity(type: 'note_added', actorMembershipId: _me, body: 'x'),
        myMembershipId: null,
      );
      expect(entry.authorName, 'Anggota tim lain');
    });
  });

  group('lostReasonDisplayLabel', () {
    test('returns null for a null reason', () {
      expect(lostReasonDisplayLabel(null), isNull);
    });

    test('returns the Indonesian label for a known reason', () {
      expect(lostReasonDisplayLabel('price'), 'Harga');
    });
  });
}
