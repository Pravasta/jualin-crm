import 'package:crm_employee/features/leads/data/datasources/lead_remote_data_source.dart';
import 'package:flutter_test/flutter_test.dart';

// Issue #152. Without per_page crm_be returns its default 25, and Lead Saya
// showed no more than that — silently. The path is also the offline cache key.
void main() {
  test('per_page=100 is always sent, with or without filters', () {
    expect(leadsListPath(), '/v1/leads?per_page=100');
    expect(leadsListPath(status: 'new'), '/v1/leads?per_page=100&status=new');
    expect(leadsListPath(query: ' budi '), '/v1/leads?per_page=100&q=budi');
  });
}
