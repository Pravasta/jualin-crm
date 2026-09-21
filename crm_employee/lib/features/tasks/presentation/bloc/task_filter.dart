/// Which half of Tugas Saya is showing (issue #148). Until then the screen
/// only ever asked for `status=open`, so a task disappeared the moment it
/// was completed — no history anywhere on the phone.
enum TaskFilter {
  open('open', 'Belum selesai'),
  done('done', 'Selesai');

  /// The `status` value `GET /v1/tasks` takes.
  final String status;
  final String label;

  const TaskFilter(this.status, this.label);

  /// crm_be defaults to 25 per page and caps at 100. History is the list
  /// that keeps growing, so Selesai asks for the cap; Belum selesai keeps
  /// the default it has always used.
  int? get perPage => this == TaskFilter.done ? 100 : null;
}
