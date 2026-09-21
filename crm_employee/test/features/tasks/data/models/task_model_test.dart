import 'package:crm_employee/features/tasks/data/models/task_model.dart';
import 'package:flutter_test/flutter_test.dart';

// Issue #148: the Selesai tab shows WHEN a task was completed, from the
// completed_at crm_be already sends (internal/task/handler_http.go).
void main() {
  Map<String, dynamic> json({Object? completedAt}) => {
        'id': 't1',
        'lead_id': 'l1',
        'title': 'Follow up',
        'status': 'done',
        'version': 2,
        'completed_at': completedAt,
      };

  test('reads completed_at', () {
    final task = TaskModel.fromJson(json(completedAt: '2026-09-20T08:30:00Z'));
    expect(task.completedAt, DateTime.utc(2026, 9, 20, 8, 30));
  });

  test('an open task has no completed_at', () {
    expect(TaskModel.fromJson(json(completedAt: null)).completedAt, isNull);
  });
}
