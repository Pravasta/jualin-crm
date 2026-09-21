import 'package:crm_employee/shared/truncation_notice.dart';
import 'package:flutter_test/flutter_test.dart';

// Issue #152. Lead Saya and Tugas Saya used to show only crm_be's default 25,
// with nothing on screen to say more existed. They now load 100 and, when the
// server's meta.total is larger, SAY so — including which ones are missing:
// crm_be orders by created_at DESC, so it is always the oldest that are cut.
void main() {
  test('nothing to say when everything fits', () {
    expect(truncationNotice(shown: 40, total: 40, noun: 'lead'), isNull);
    expect(truncationNotice(shown: 0, total: 0, noun: 'tugas'), isNull);
  });

  test('names how many are shown, out of how many, and which are missing', () {
    expect(
      truncationNotice(shown: 100, total: 134, noun: 'lead'),
      'Menampilkan 100 dari 134 lead. 34 lead terlama tidak ditampilkan.',
    );
  });

  test('appends the hint when there is one', () {
    expect(
      truncationNotice(shown: 100, total: 101, noun: 'lead', hint: 'Persempit dengan status atau pencarian.'),
      'Menampilkan 100 dari 101 lead. 1 lead terlama tidak ditampilkan. Persempit dengan status atau pencarian.',
    );
  });

  test('never claims a negative remainder if the total is stale or smaller', () {
    // Offline cache or a race can make total < shown; say nothing rather
    // than "-3 lead terlama".
    expect(truncationNotice(shown: 100, total: 97, noun: 'tugas'), isNull);
  });
}
