import 'package:crm_employee/features/tasks/domain/entities/task.dart';
import 'package:crm_employee/features/tasks/presentation/widgets/task_list_item.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

// Issue #153. due_label_test.dart proves dueLabel is right; this proves the
// ROW actually uses it. Without it, pointing the row back at relativeTime —
// the original bug — kept every test green. Dates are far from now on
// purpose, so the assertions do not depend on the time of day the test runs.
void main() {
  Future<void> pumpRow(WidgetTester tester, Task task) {
    return tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: TaskListItem(task: task, isCompleting: false, onComplete: () {}, onTap: () {}),
        ),
      ),
    );
  }

  Task task({required DateTime dueAt}) =>
      Task(id: 't1', leadId: 'l1', title: 'Follow up', dueAt: dueAt, status: 'open', version: 1);

  testWidgets('a task due in the future never reads "Baru saja"', (tester) async {
    await pumpRow(tester, task(dueAt: DateTime.now().add(const Duration(days: 3))));

    expect(find.textContaining('Baru saja'), findsNothing);
    expect(find.text('Jatuh tempo dalam 3 hari'), findsOneWidget);
  });

  testWidgets('an overdue task says so in words', (tester) async {
    await pumpRow(tester, task(dueAt: DateTime.now().subtract(const Duration(days: 4))));

    expect(find.text('Terlambat 4 hari'), findsOneWidget);
  });
}
