/// "When is this task due" — the question [relativeTime] does not answer.
///
/// [relativeTime] (relative_time.dart) only handles the past ("disentuh 2j
/// lalu"): for a future date its difference is negative and it returned "Baru
/// saja", so a task due next week read "Jatuh tempo Baru saja" (issue #153).
/// It is kept as it is — four other callers rely on it for the past — and this
/// is a separate function for a separate question.
///
/// Counted in LOCAL CALENDAR DAYS, not 24-hour blocks: a task due at 08:00
/// tomorrow, seen at 23:30 tonight, is "besok", not "dalam 8 jam". It says
/// "Terlambat" exactly when [dueAt] is before [now] — the same test that paints
/// the row red in task_list_item.dart, so the words and the colour agree.
///
/// [now] is injectable so tests don't depend on the wall clock.
String dueLabel(DateTime dueAt, {DateTime? now}) {
  final reference = (now ?? DateTime.now()).toLocal();
  final due = dueAt.toLocal();

  final days = _calendarDay(due).difference(_calendarDay(reference)).inDays;

  if (due.isBefore(reference)) {
    final late = -days;
    return late <= 0 ? 'Terlambat' : 'Terlambat $late hari';
  }
  if (days == 0) return 'Jatuh tempo hari ini';
  if (days == 1) return 'Jatuh tempo besok';
  if (days <= 7) return 'Jatuh tempo dalam $days hari';

  final date = '${due.day} ${_months[due.month - 1]}';
  return due.year == reference.year ? 'Jatuh tempo $date' : 'Jatuh tempo $date ${due.year}';
}

// UTC midnight of the local calendar date: difference().inDays between two of
// these is exact, with no daylight-saving hour to round away.
DateTime _calendarDay(DateTime t) => DateTime.utc(t.year, t.month, t.day);

const _months = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des'];
