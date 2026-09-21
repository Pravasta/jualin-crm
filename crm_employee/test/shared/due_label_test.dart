import 'package:crm_employee/shared/due_label.dart';
import 'package:flutter_test/flutter_test.dart';

// Issue #153. relativeTime only answers "how long ago" — for a due date in
// the future its difference is negative and it returned "Baru saja", so a task
// due next week read "Jatuh tempo Baru saja". dueLabel answers the other
// question, "when is it due", counted in LOCAL CALENDAR DAYS: a task due at
// 08:00 tomorrow seen at 23:30 tonight is "besok", not "dalam 8 jam".
//
// All times here are local DateTimes (no Z), so the tests do not depend on the
// machine's timezone.
void main() {
  final now = DateTime(2026, 9, 21, 14, 0); // Senin 21 Sep 2026, 14:00

  group('upcoming', () {
    test('later today', () {
      expect(dueLabel(DateTime(2026, 9, 21, 17, 0), now: now), 'Jatuh tempo hari ini');
    });

    test('tomorrow', () {
      expect(dueLabel(DateTime(2026, 9, 22, 9, 0), now: now), 'Jatuh tempo besok');
    });

    test('a few days out', () {
      expect(dueLabel(DateTime(2026, 9, 24), now: now), 'Jatuh tempo dalam 3 hari');
      expect(dueLabel(DateTime(2026, 9, 28), now: now), 'Jatuh tempo dalam 7 hari');
    });

    test('beyond a week: an absolute date, month in Indonesian', () {
      expect(dueLabel(DateTime(2026, 10, 12), now: now), 'Jatuh tempo 12 Okt');
      expect(dueLabel(DateTime(2026, 12, 3), now: now), 'Jatuh tempo 3 Des');
    });

    test('another year: the year is shown too', () {
      expect(dueLabel(DateTime(2027, 1, 5), now: now), 'Jatuh tempo 5 Jan 2027');
    });

    test('never "Baru saja" for any future date — the bug', () {
      for (final d in [1, 2, 5, 10, 40, 400]) {
        expect(
          dueLabel(now.add(Duration(days: d)), now: now),
          isNot(contains('Baru saja')),
          reason: '+$d days',
        );
      }
    });
  });

  group('overdue', () {
    test('earlier today, already passed', () {
      expect(dueLabel(DateTime(2026, 9, 21, 9, 0), now: now), 'Terlambat');
    });

    test('yesterday and older', () {
      expect(dueLabel(DateTime(2026, 9, 20, 18, 0), now: now), 'Terlambat 1 hari');
      expect(dueLabel(DateTime(2026, 9, 14), now: now), 'Terlambat 7 hari');
    });
  });

  group('midnight boundary — calendar days, not 24-hour blocks', () {
    test('late tonight, due early tomorrow morning: besok', () {
      final lateNight = DateTime(2026, 9, 21, 23, 30);
      expect(dueLabel(DateTime(2026, 9, 22, 8, 0), now: lateNight), 'Jatuh tempo besok');
    });

    test('just after midnight, due later the same day: hari ini', () {
      final earlyMorning = DateTime(2026, 9, 22, 0, 30);
      expect(dueLabel(DateTime(2026, 9, 22, 23, 0), now: earlyMorning), 'Jatuh tempo hari ini');
    });

    test('just after midnight, due late last night: Terlambat 1 hari', () {
      final earlyMorning = DateTime(2026, 9, 22, 0, 30);
      expect(dueLabel(DateTime(2026, 9, 21, 23, 0), now: earlyMorning), 'Terlambat 1 hari');
    });
  });

  // The row is painted red exactly when dueAt is before now
  // (task_list_item.dart). The words must agree with the colour.
  test('says "Terlambat" exactly when the task is overdue', () {
    for (final minutes in [-3000, -61, -1, 1, 61, 3000]) {
      final due = now.add(Duration(minutes: minutes));
      final overdue = due.isBefore(now);
      expect(dueLabel(due, now: now).startsWith('Terlambat'), overdue, reason: '$minutes min');
    }
  });
}
