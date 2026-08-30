// Proves NotificationRepositoryImpl's orchestration of
// NotificationRemoteDataSource — deliberately no cache-fallback tests
// here, unlike lead_repository_impl_test.dart/task_repository_impl_test.dart:
// this endpoint isn't one of TD §7's four cacheable ones (see
// NotificationListResult's own doc comment).
import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/features/notifications/data/datasources/notification_remote_data_source.dart';
import 'package:crm_employee/features/notifications/data/repositories/notification_repository_impl.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockNotificationRemoteDataSource extends Mock
    implements NotificationRemoteDataSource {}

const _list = [
  {
    'id': 'n1',
    'type': 'lead_assigned',
    'lead_id': 'l1',
    'task_id': null,
    'title': 'Lead baru',
    'body': 'Rina Wijaya ditugaskan ke Anda',
    'read_at': null,
    'created_at': '2026-08-30T00:00:00Z',
  },
];

void main() {
  late MockNotificationRemoteDataSource remoteDataSource;
  late NotificationRepositoryImpl repository;

  setUp(() {
    remoteDataSource = MockNotificationRemoteDataSource();
    repository = NotificationRepositoryImpl(remoteDataSource: remoteDataSource);
  });

  group('getNotifications', () {
    test('parses the list, unread stays unread (null read_at)', () async {
      when(() => remoteDataSource.list()).thenAnswer((_) async => _list);

      final result = await repository.getNotifications();

      result.fold((f) => fail('expected Right, got Left(${f.message})'), (
        list,
      ) {
        expect(list.notifications, hasLength(1));
        expect(list.notifications.first.isUnread, isTrue);
        expect(list.notifications.first.leadId, 'l1');
      });
    });

    test('a real ApiError surfaces as a Failure, not a crash', () async {
      when(() => remoteDataSource.list()).thenThrow(
        const ApiError(status: 500, code: 'internal_error', message: 'Terjadi kesalahan internal.'),
      );

      final result = await repository.getNotifications();

      expect(result.isLeft(), isTrue);
    });
  });

  group('markRead', () {
    test('forwards to the data source and returns Right on success', () async {
      when(() => remoteDataSource.markRead('n1')).thenAnswer((_) async {});

      final result = await repository.markRead('n1');

      expect(result.isRight(), isTrue);
      verify(() => remoteDataSource.markRead('n1')).called(1);
    });
  });
}
