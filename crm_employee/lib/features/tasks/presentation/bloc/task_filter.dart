import '../../../../shared/truncation_notice.dart';

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

  /// Both tabs ask for [kListPageSize] (issue #152). Belum selesai used to
  /// send nothing and silently got crm_be's default 25 — the OLDEST open
  /// tasks, the ones most likely overdue, were the ones cut.
  int get perPage => kListPageSize;
}
