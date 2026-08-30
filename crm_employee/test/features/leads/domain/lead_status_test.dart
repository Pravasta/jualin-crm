import 'package:crm_employee/features/leads/domain/lead_status.dart';
import 'package:crm_employee/shared/labels.dart';
import 'package:flutter_test/flutter_test.dart';

// This is the literal transition matrix from
// crm_be/internal/lead/usecase.go's validateStatusTransition, ported
// verbatim from crm_dashboard/src/lib/lead-status.test.ts (itself worked
// out by hand from the Go source, not derived from
// isValidStatusTransition) — so a bug that breaks BOTH ports the same
// way still can't hide.
const Map<String, List<String>> _expectedValid = {
  'new': ['contacted', 'lost', 'unqualified', 'spam'],
  'contacted': ['new', 'qualified', 'lost', 'unqualified', 'spam'],
  'qualified': ['contacted', 'proposal', 'lost', 'unqualified', 'spam'],
  'proposal': ['qualified', 'won', 'lost', 'unqualified', 'spam'],
  'won': ['proposal', 'lost', 'unqualified', 'spam'],
  // "leaving lost" is a documented backend simplification: ANY main-path
  // status is valid, not just the one the lead was in before — there's
  // no cheap way to know "before" without activity history (crm_be #20
  // notes). unqualified/spam are also reachable directly from lost.
  'lost': [
    'new',
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
      expect(isValidStatusTransition('proposal', 'new'), isFalse); // two steps back
      expect(isValidStatusTransition('new', 'won'), isFalse); // skips ahead
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

    test('restricts "lost" to a single reopen-to-"new" option, not all 5 valid main-path targets', () {
      // isValidStatusTransition allows lost -> any main-path status; the
      // UI deliberately narrows this to avoid a wall of buttons for a
      // rare case. This test locks that narrowing as intentional.
      final options = statusTransitionOptions('lost');
      expect(options, hasLength(1));
      expect(options[0].status, 'new');
    });

    test('every option returned is actually valid per isValidStatusTransition — AC #72: UI never offers what the backend rejects', () {
      for (final from in leadStatuses) {
        for (final option in statusTransitionOptions(from)) {
          expect(isValidStatusTransition(from, option.status), isTrue);
        }
      }
    });
  });
}
