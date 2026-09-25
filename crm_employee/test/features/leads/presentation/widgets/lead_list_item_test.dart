import 'package:crm_employee/features/leads/domain/entities/lead.dart';
import 'package:crm_employee/features/leads/presentation/widgets/lead_list_item.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

// #173 — the Lead Saya card carries what the brief asks of a row: name,
// #number, when last touched, the status badge (label + icon, not color
// alone), and "Belum ada kontak" only when there is none.
void main() {
  Lead lead({String? phone, String? email}) => Lead(
        id: 'l1',
        leadNumber: 1024,
        name: 'Dewi Lestari',
        email: email,
        phone: phone,
        status: 'proposal',
        source: 'form',
        version: 1,
        createdAt: DateTime.now().toUtc(),
        updatedAt: DateTime.now().toUtc().subtract(const Duration(hours: 2)),
      );

  Future<void> pump(WidgetTester tester, Lead l) => tester.pumpWidget(
        MaterialApp(home: Scaffold(body: LeadListItem(lead: l, onTap: () {}))),
      );

  testWidgets('name, number, touched time and the status badge', (tester) async {
    await pump(tester, lead(phone: '0812-3456-7890'));

    expect(find.text('Dewi Lestari'), findsOneWidget);
    expect(find.textContaining('#1024 · disentuh'), findsOneWidget);
    expect(find.text('Penawaran'), findsOneWidget);
    expect(find.byIcon(Icons.description_outlined), findsOneWidget);
    expect(find.text('Belum ada kontak'), findsNothing);
  });

  testWidgets('no email and no phone: says so', (tester) async {
    await pump(tester, lead());
    expect(find.text('Belum ada kontak'), findsOneWidget);
  });
}
