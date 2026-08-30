import 'package:dartz/dartz.dart';
import 'package:firebase_messaging/firebase_messaging.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/network/run_api_call.dart';
import '../../../../core/push/device_token_remote_data_source.dart';
import '../../../../core/push/push_token_store.dart';
import '../../domain/entities/push_message.dart';
import '../../domain/repositories/push_repository.dart';
import '../datasources/firebase_messaging_data_source.dart';

class PushRepositoryImpl implements PushRepository {
  final FirebaseMessagingDataSource messagingDataSource;
  final DeviceTokenRemoteDataSource deviceTokenRemoteDataSource;
  final PushTokenStore pushTokenStore;

  const PushRepositoryImpl({
    required this.messagingDataSource,
    required this.deviceTokenRemoteDataSource,
    required this.pushTokenStore,
  });

  @override
  Future<String?> getFcmToken() => messagingDataSource.getToken();

  @override
  Future<bool> requestPermission() => messagingDataSource.requestPermission();

  @override
  Stream<String> get onTokenRefresh => messagingDataSource.onTokenRefresh;

  @override
  Stream<PushMessage> get onForegroundMessage =>
      messagingDataSource.onMessage.map(_toPushMessage);

  @override
  Stream<PushMessage> get onMessageOpenedApp =>
      messagingDataSource.onMessageOpenedApp.map(_toPushMessage);

  @override
  Future<PushMessage?> getInitialMessage() async {
    final message = await messagingDataSource.getInitialMessage();
    return message == null ? null : _toPushMessage(message);
  }

  @override
  Future<Either<Failure, void>> registerToken(String token) {
    return runApiCall(() async {
      await deviceTokenRemoteDataSource.register(
        token: token,
        // Android only (Phase 5 decision M1) — never a value the
        // backend needs to infer, `internal/device`'s own `platform`
        // column just records what the client asserts.
        platform: 'android',
      );
      // So AuthRepositoryImpl.logout() knows what to unregister later —
      // see device_token_remote_data_source.dart's doc comment.
      await pushTokenStore.save(token);
    });
  }

  PushMessage _toPushMessage(RemoteMessage message) {
    return PushMessage(
      leadId: message.data['lead_id'] as String?,
      title: message.notification?.title,
      body: message.notification?.body,
    );
  }
}
