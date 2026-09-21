import 'package:crm_employee/features/tasks/data/datasources/task_remote_data_source.dart';
import 'package:flutter_test/flutter_test.dart';

// The path is also the offline cache key (TaskRepositoryImpl), so Belum
// selesai and Selesai MUST produce different paths — otherwise one tab's
// cached list would be served as the other's in airplane mode (issue #148).
void main() {
  test('open and done are different paths, so they cache separately', () {
    final open = tasksListPath(assignedTo: 'm1', status: 'open');
    final done = tasksListPath(assignedTo: 'm1', status: 'done', perPage: 100);
    expect(open, isNot(done));
    expect(open, '/v1/tasks?assigned_to=m1&status=open');
    expect(done, '/v1/tasks?assigned_to=m1&status=done&per_page=100');
  });

  test('per_page is only sent when asked for — Belum selesai keeps the server default', () {
    expect(tasksListPath(assignedTo: 'm1', status: 'open'), isNot(contains('per_page')));
  });
}
