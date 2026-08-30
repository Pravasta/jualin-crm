import 'package:firebase_messaging/firebase_messaging.dart';

/// Wraps `FirebaseMessaging.instance`/its static streams — no real
/// platform channel in `flutter test`'s host environment, same reasoning
/// `BiometricLocalDataSource` (#69) and `ExternalAppDataSource` (#72)
/// wrap their own plugins. `PushRepositoryImpl` is what maps
/// `RemoteMessage` into the domain's `PushMessage`; this layer only
/// knows how to talk to the plugin.
abstract class FirebaseMessagingDataSource {
  Future<String?> getToken();

  /// `true` for authorized or provisional — see
  /// `PushRepository.requestPermission`'s doc comment.
  Future<bool> requestPermission();

  Stream<String> get onTokenRefresh;
  Stream<RemoteMessage> get onMessage;
  Stream<RemoteMessage> get onMessageOpenedApp;
  Future<RemoteMessage?> getInitialMessage();
}

class FirebaseMessagingDataSourceImpl implements FirebaseMessagingDataSource {
  final FirebaseMessaging _messaging;

  FirebaseMessagingDataSourceImpl({FirebaseMessaging? messaging})
    : _messaging = messaging ?? FirebaseMessaging.instance;

  @override
  Future<String?> getToken() => _messaging.getToken();

  @override
  Future<bool> requestPermission() async {
    final settings = await _messaging.requestPermission();
    return settings.authorizationStatus == AuthorizationStatus.authorized ||
        settings.authorizationStatus == AuthorizationStatus.provisional;
  }

  @override
  Stream<String> get onTokenRefresh => _messaging.onTokenRefresh;

  // Static on the plugin, not the instance — firebase_messaging's own
  // API shape, not a choice made here.
  @override
  Stream<RemoteMessage> get onMessage => FirebaseMessaging.onMessage;

  @override
  Stream<RemoteMessage> get onMessageOpenedApp =>
      FirebaseMessaging.onMessageOpenedApp;

  @override
  Future<RemoteMessage?> getInitialMessage() => _messaging.getInitialMessage();
}
