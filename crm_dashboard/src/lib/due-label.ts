// "When is this task due" in calendar words — brief §8.7: hari ini / besok /
// dalam 3 hari / 12 Okt / Terlambat 2 hari. A port of the mobile app's
// crm_employee/lib/shared/due_label.dart (issue #153), rule for rule, so a
// task reads the same on the dashboard and on the phone (#165).
//
// Counted in LOCAL CALENDAR DAYS, not 24-hour blocks: a task due at 08:00
// tomorrow, seen at 23:30 tonight, is "besok", not "dalam 8 jam". It says
// "Terlambat" exactly when dueAt is before now — the same test that paints
// the row, so the words and the color agree.
//
// `now` is injectable so tests don't depend on the wall clock.

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];

// UTC midnight of the local calendar date: the difference between two of
// these is a whole number of days, with no daylight-saving hour to round.
function calendarDay(t: Date): number {
  return Date.UTC(t.getFullYear(), t.getMonth(), t.getDate());
}

export function isOverdue(dueAt: string | Date, now: Date = new Date()): boolean {
  return new Date(dueAt).getTime() < now.getTime();
}

export function dueLabel(dueAt: string | Date, now: Date = new Date()): string {
  const due = new Date(dueAt);
  const days = Math.round((calendarDay(due) - calendarDay(now)) / 86_400_000);

  if (isOverdue(due, now)) {
    const late = -days;
    return late <= 0 ? "Terlambat" : `Terlambat ${late} hari`;
  }
  if (days === 0) return "Jatuh tempo hari ini";
  if (days === 1) return "Jatuh tempo besok";
  if (days <= 7) return `Jatuh tempo dalam ${days} hari`;

  const date = `${due.getDate()} ${MONTHS[due.getMonth()]}`;
  return due.getFullYear() === now.getFullYear() ? `Jatuh tempo ${date}` : `Jatuh tempo ${date} ${due.getFullYear()}`;
}
