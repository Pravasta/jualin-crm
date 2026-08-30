import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/material.dart';

import 'app.dart';
import 'core/di/injection_container.dart';
import 'firebase_options.dart';

/// Required by `firebase_messaging` for background/terminated message
/// handling to even register — a top-level, `@pragma('vm:entry-point')`
/// function so the plugin can invoke it from its own background isolate.
/// Deliberately minimal: `crm_be` sends BOTH a `notification` block
/// (title/body) and a `data` block (`type`/`lead_id`) — confirmed
/// directly in `internal/shared/push/fcm.go`. Android displays the tray
/// notification from the `notification` block automatically while
/// backgrounded; this handler doesn't need to build anything itself, only
/// exist. Navigation on tap is `onMessageOpenedApp`'s job
/// (`PushBloc`), not this.
@pragma('vm:entry-point')
Future<void> _firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  await Firebase.initializeApp(options: DefaultFirebaseOptions.currentPlatform);
}

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Firebase.initializeApp(options: DefaultFirebaseOptions.currentPlatform);
  FirebaseMessaging.onBackgroundMessage(_firebaseMessagingBackgroundHandler);
  await initDependencyInjection();
  runApp(const App());
}
