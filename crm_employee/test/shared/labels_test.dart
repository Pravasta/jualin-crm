// Locks shared/labels.dart's enum lists to crm_be's own CHECK
// constraints (crm_be/migrations/0002_identity.sql,
// crm_be/migrations/0003_crm_core.sql) — a drift here means a status/
// role/source/reason the backend accepts silently has no Indonesian
// label anywhere in the app.
import 'package:crm_employee/shared/labels.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('leadStatuses', () {
    test('8 values, matching ck_leads_status exactly', () {
      expect(leadStatuses, hasLength(8));
      expect(leadStatuses, [
        'new',
        'contacted',
        'qualified',
        'proposal',
        'won',
        'lost',
        'unqualified',
        'spam',
      ]);
    });

    test('every status has a StatusMeta entry with a non-empty label', () {
      for (final status in leadStatuses) {
        final meta = statusMeta[status];
        expect(meta, isNotNull, reason: 'missing StatusMeta for "$status"');
        expect(meta!.label, isNotEmpty);
      }
    });

    test('statusMeta has no entries beyond the 8 known statuses', () {
      expect(statusMeta.keys.toSet(), leadStatuses.toSet());
    });
  });

  group('lostReasons', () {
    test('6 values, matching ck_leads_lost_reason exactly', () {
      expect(lostReasons, hasLength(6));
      expect(lostReasons, [
        'price',
        'competitor',
        'timing',
        'no_response',
        'not_interested',
        'other',
      ]);
    });

    test('every reason has a non-empty label', () {
      for (final reason in lostReasons) {
        expect(lostReasonLabels[reason], isNotNull);
        expect(lostReasonLabels[reason], isNotEmpty);
      }
    });
  });

  group('leadSources', () {
    test('4 values, matching ck_leads_source exactly', () {
      expect(leadSources, hasLength(4));
      expect(leadSources, ['manual', 'api', 'form', 'webhook']);
    });

    test('every source has a non-empty label', () {
      for (final source in leadSources) {
        expect(sourceLabels[source], isNotNull);
        expect(sourceLabels[source], isNotEmpty);
      }
    });
  });

  group('roles', () {
    test('4 values, matching ck_memberships_role exactly', () {
      expect(roles, hasLength(4));
      expect(roles, ['owner', 'admin', 'manager', 'employee']);
    });

    test('every role has a non-empty label', () {
      for (final role in roles) {
        expect(roleLabels[role], isNotNull);
        expect(roleLabels[role], isNotEmpty);
      }
    });
  });
}
