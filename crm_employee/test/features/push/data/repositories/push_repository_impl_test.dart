// Proves PushRepositoryImpl's orchestration: mapping RemoteMessage ->
// PushMessage, and registerToken's two side effects (POST + local
// store write) happening together.
import 'dart:async';

import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/core/push/device_token_remote_data_source.dart';
import 'package:crm_employee/core/push/push_token_store.dart';
import 'package:crm_employee/features/push/data/datasources/firebase_messaging_data_source.dart';
import 'package:crm_employee/features/push/data/repositories/push_repository_impl.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockFirebaseMessagingDataSource extends Mock
    implements FirebaseMessagingDataSource {}

class MockDeviceTokenRemoteDataSource extends Mock
    implements DeviceTokenRemoteDataSource {}

class MockPushTokenStore extends Mock implements PushTokenStore {}

class FakeRemoteMessage extends Fake implements RemoteMessage {
  @override
  final Map<String, dynamic> data;
  @override
  final RemoteNotification? notification;

  FakeRemoteMessage({this.data = const {}, this.notification});
}

class FakeRemoteNotification extends Fake implements RemoteNotification {
  @override
  final String? title;
  @override
  final String? body;

  FakeRemoteNotification({this.title, this.body});
}

void main() {
  late MockFirebaseMessagingDataSource messagingDataSource;
  late MockDeviceTokenRemoteDataSource deviceTokenRemoteDataSource;
  late MockPushTokenStore pushTokenStore;
  late PushRepositoryImpl repository;

  setUp(() {
    messagingDataSource = MockFirebaseMessagingDataSource();
    deviceTokenRemoteDataSource = MockDeviceTokenRemoteDataSource();
    pushTokenStore = MockPushTokenStore();
    repository = PushRepositoryImpl(
      messagingDataSource: messagingDataSource,
      deviceTokenRemoteDataSource: deviceTokenRemoteDataSource,
      pushTokenStore: pushTokenStore,
    );
  });

  group('getFcmToken / requestPermission', () {
    test('forwards to the data source unchanged', () async {
      when(() => messagingDataSource.getToken()).thenAnswer((_) async => 'tok-1');
      when(
        () => messagingDataSource.requestPermission(),
      ).thenAnswer((_) async => true);

      expect(await repository.getFcmToken(), 'tok-1');
      expect(await repository.requestPermission(), isTrue);
    });
  });

  group('onForegroundMessage / onMessageOpenedApp', () {
    test('maps data.lead_id and notification title/body into PushMessage', () async {
      final controller = StreamController<RemoteMessage>();
      when(() => messagingDataSource.onMessage).thenAnswer((_) => controller.stream);

      final future = repository.onForegroundMessage.first;
      controller.add(
        FakeRemoteMessage(
          data: {'type': 'lead_assigned', 'lead_id': 'l1'},
          notification: FakeRemoteNotification(
            title: 'Lead baru',
            body: 'Rina Wijaya ditugaskan ke Anda',
          ),
        ),
      );
      final result = await future;

      expect(result.leadId, 'l1');
      expect(result.title, 'Lead baru');
      expect(result.body, 'Rina Wijaya ditugaskan ke Anda');
      await controller.close();
    });

    test('a message with no lead_id maps to a null leadId, never throws', () async {
      final controller = StreamController<RemoteMessage>();
      when(
        () => messagingDataSource.onMessageOpenedApp,
      ).thenAnswer((_) => controller.stream);

      final future = repository.onMessageOpenedApp.first;
      controller.add(FakeRemoteMessage());
      final result = await future;

      expect(result.leadId, isNull);
      await controller.close();
    });
  });

  group('getInitialMessage', () {
    test('null when the app was not launched from a notification', () async {
      when(
        () => messagingDataSource.getInitialMessage(),
      ).thenAnswer((_) async => null);

      expect(await repository.getInitialMessage(), isNull);
    });

    test('maps the launching message when present', () async {
      when(() => messagingDataSource.getInitialMessage()).thenAnswer(
        (_) async => FakeRemoteMessage(data: {'lead_id': 'l2'}),
      );

      final result = await repository.getInitialMessage();

      expect(result?.leadId, 'l2');
    });
  });

  group('registerToken', () {
    test('registers with the backend AND saves the token locally — both, not just one', () async {
      when(
        () => deviceTokenRemoteDataSource.register(
          token: 'tok-1',
          platform: 'android',
        ),
      ).thenAnswer((_) async {});
      when(() => pushTokenStore.save('tok-1')).thenAnswer((_) async {});

      final result = await repository.registerToken('tok-1');

      expect(result.isRight(), isTrue);
      verify(
        () => deviceTokenRemoteDataSource.register(
          token: 'tok-1',
          platform: 'android',
        ),
      ).called(1);
      verify(() => pushTokenStore.save('tok-1')).called(1);
    });

    test('a real ApiError surfaces as a Failure, never saved locally as if it succeeded', () async {
      when(
        () => deviceTokenRemoteDataSource.register(
          token: 'tok-1',
          platform: 'android',
        ),
      ).thenThrow(
        const ApiError(status: 500, code: 'internal_error', message: 'Terjadi kesalahan internal.'),
      );

      final result = await repository.registerToken('tok-1');

      expect(result.isLeft(), isTrue);
      verifyNever(() => pushTokenStore.save(any()));
    });
  });
}
