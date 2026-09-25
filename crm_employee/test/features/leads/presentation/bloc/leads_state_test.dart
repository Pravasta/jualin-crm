import 'package:crm_employee/features/leads/presentation/bloc/leads_state.dart';
import 'package:flutter_test/flutter_test.dart';

// #173 — an empty Lead Saya is one of two different answers (brief §13):
// "belum ada lead ditugaskan" vs "tidak ada yang cocok" with a way out.
void main() {
  test('not filtered: no status chip and no search', () {
    expect(const LeadsLoading().isFiltered, isFalse);
    expect(const LeadsLoading(query: '   ').isFiltered, isFalse);
  });

  test('a status chip or a search counts as filtered', () {
    expect(const LeadsLoading(statusFilter: 'new').isFiltered, isTrue);
    expect(const LeadsLoading(query: 'dewi').isFiltered, isTrue);
  });
}
