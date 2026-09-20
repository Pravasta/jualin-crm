import 'package:crm_employee/features/leads/domain/entities/activity.dart';
import 'package:crm_employee/features/leads/domain/lead_status.dart';
import 'package:crm_employee/shared/labels.dart';
import 'package:flutter_test/flutter_test.dart';

// This is the literal transition matrix from
// crm_be/internal/lead/usecase.go's validateStatusTransition, ported
// verbatim from crm_dashboard/src/lib/lead-status.test.ts (itself worked
// out by hand from the Go source, not derived from
// isValidStatusTransition) — so a bug that breaks BOTH ports the same
// way still can't hide.
//
// Nothing leads to 'new' (issue #139, ADR-015): it means "belum disentuh",
// a statement about history rather than a stage of work, so no row below
// lists it — not even lost's, which is why reopening goes to 'contacted'.
const Map<String, List<String>> _expectedValid = {
  'new': ['contacted', 'lost', 'unqualified', 'spam'],
  'contacted': ['qualified', 'lost', 'unqualified', 'spam'],
  'qualified': ['contacted', 'proposal', 'lost', 'unqualified', 'spam'],
  'proposal': ['qualified', 'won', 'lost', 'unqualified', 'spam'],
  'won': ['proposal', 'lost', 'unqualified', 'spam'],
  // "leaving lost" is a documented backend simplification: ANY main-path
  // status is valid, not just the one the lead was in before — there's
  // no cheap way to know "before" without activity history (crm_be #20
  // notes). unqualified/spam are also reachable directly from lost.
  'lost': [
    'contacted',
    'qualified',
    'proposal',
    'won',
    'unqualified',
    'spam',
  ],
  'unqualified': [],
  'spam': [],
};

void main() {
  group('isValidStatusTransition', () {
    test('matches the full matrix for every (from, to) pair — including same-status', () {
      for (final from in leadStatuses) {
        for (final to in leadStatuses) {
          final expected = to == from
              ? to == 'lost'
              : _expectedValid[from]!.contains(to);
          expect(
            isValidStatusTransition(from, to),
            expected,
            reason: '$from -> $to expected $expected',
          );
        }
      }
    });

    test('unqualified and spam are final — no outgoing transition at all, not even to themselves', () {
      for (final to in leadStatuses) {
        expect(isValidStatusTransition('unqualified', to), isFalse);
        expect(isValidStatusTransition('spam', to), isFalse);
      }
    });

    test('main path movement is exactly one step in either direction', () {
      expect(isValidStatusTransition('qualified', 'contacted'), isTrue); // back
      expect(isValidStatusTransition('qualified', 'proposal'), isTrue); // forward
      expect(isValidStatusTransition('proposal', 'qualified'), isTrue); // back
      expect(isValidStatusTransition('won', 'proposal'), isTrue); // back
      expect(isValidStatusTransition('proposal', 'new'), isFalse); // two steps back
      expect(isValidStatusTransition('new', 'won'), isFalse); // skips ahead
    });
  });

  group('nothing leads back to new (issue #139)', () {
    test('no status can move to new — checked per status, not only via the matrix', () {
      for (final from in leadStatuses) {
        expect(
          isValidStatusTransition(from, 'new'),
          isFalse,
          reason: '$from -> new',
        );
      }
    });

    test('the UI never offers a way into new, from any status', () {
      for (final from in leadStatuses) {
        final targets = statusTransitionOptions(from).map((o) => o.status);
        expect(targets, isNot(contains('new')), reason: 'options from $from');
      }
    });

    test('"contacted" offers forward and the side exits, but no backward step', () {
      final steps = statusTransitionOptions('contacted')
          .where((o) => o.kind == StatusTransitionKind.step)
          .map((o) => o.status);
      expect(steps, ['qualified']);
    });
  });

  group('statusTransitionOptions', () {
    test('offers nothing for the two final statuses', () {
      expect(statusTransitionOptions('unqualified'), isEmpty);
      expect(statusTransitionOptions('spam'), isEmpty);
    });

    test('offers only both main-path neighbors plus the three side exits for a middle status', () {
      final options = statusTransitionOptions('qualified');
      final statuses = options.map((o) => o.status).toList()..sort();
      expect(
        statuses,
        ['contacted', 'proposal', 'lost', 'unqualified', 'spam']..sort(),
      );
    });

    test('offers only forward, no backward, for the first main-path status', () {
      final options = statusTransitionOptions('new');
      final steps = options
          .where((o) => o.kind == StatusTransitionKind.step)
          .map((o) => o.status);
      expect(steps, ['contacted']);
    });

    test('offers only backward, no forward, for the last main-path status', () {
      final options = statusTransitionOptions('won');
      final steps = options
          .where((o) => o.kind == StatusTransitionKind.step)
          .map((o) => o.status);
      expect(steps, ['proposal']);
    });

    test('restricts "lost" to a single reopen-to-"contacted" option, not all valid main-path targets', () {
      // isValidStatusTransition allows lost -> any main-path status except
      // new; the UI deliberately narrows this to avoid a wall of buttons
      // for a rare case. This test locks that narrowing as intentional —
      // and the target: 'new' is closed (ADR-015), 'contacted' is the
      // earliest stage a reopened lead can honestly be in.
      final options = statusTransitionOptions('lost');
      expect(options, hasLength(1));
      expect(options[0].status, 'contacted');
      expect(options[0].label, '→ Buka kembali ke Dihubungi');
    });

    test('every option returned is actually valid per isValidStatusTransition — AC #72: UI never offers what the backend rejects', () {
      for (final from in leadStatuses) {
        for (final option in statusTransitionOptions(from)) {
          expect(isValidStatusTransition(from, option.status), isTrue);
        }
      }
    });
  });

  // Issue #142. The timeline's `lead_converted` entry is the only "already
  // converted" signal the client has (the Lead carries no such flag), so the
  // predicate that drives "stop offering the status picker" is worth pinning.
  group('hasBeenConverted', () {
    Activity entry(String type) => Activity(
          id: 'a-$type',
          leadId: 'lead-1',
          type: type,
          createdAt: DateTime.utc(2026, 9, 21),
        );

    test('is true once the timeline carries a lead_converted entry, wherever it sits', () {
      expect(hasBeenConverted([entry('lead_created'), entry('lead_converted')]), isTrue);
      expect(hasBeenConverted([entry('lead_converted'), entry('status_changed')]), isTrue);
    });

    test('is false for a timeline without one — including a won lead that was never converted', () {
      expect(hasBeenConverted(const []), isFalse);
      expect(
        hasBeenConverted([entry('lead_created'), entry('status_changed'), entry('note')]),
        isFalse,
      );
    });
  });
}
