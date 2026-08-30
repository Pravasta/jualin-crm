import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../entities/push_message.dart';

abstract class PushRepository {
  /// `null` when the plugin hasn't produced a token yet (rare — usually
  /// resolves within the first frame) or on a platform/emulator with no
  /// Google Play services.
  Future<String?> getFcmToken();

  /// Android 13+'s runtime notification permission. `true` for
  /// authorized OR provisional — both mean the OS will actually show a
  /// tray notification; anything else means it silently won't, even
  /// though the FCM message itself still arrives.
  Future<bool> requestPermission();

  /// A new token replaces the old one (device restore, app data clear,
  /// token rotation) — the old one still registered server-side becomes
  /// dead weight until a failed push against it triggers `internal/
  /// shared/push`'s existing cleanup (#68). Re-registering promptly
  /// keeps that window short.
  Stream<String> get onTokenRefresh;

  /// App was already open when the push arrived — design brief §10:
  /// render an in-app banner, never navigate away from what the user
  /// was doing.
  Stream<PushMessage> get onForegroundMessage;

  /// App was backgrounded and the user tapped the system tray
  /// notification.
  Stream<PushMessage> get onMessageOpenedApp;

  /// The message that launched the app from fully closed, if any — a
  /// ONE-TIME check, not a stream (`firebase_messaging`'s own contract:
  /// this only ever answers for the very first call after a cold
  /// start).
  Future<PushMessage?> getInitialMessage();

  Future<Either<Failure, void>> registerToken(String token);
}
