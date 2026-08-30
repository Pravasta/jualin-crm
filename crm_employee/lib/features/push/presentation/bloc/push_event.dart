import 'package:equatable/equatable.dart';

import '../../domain/entities/push_message.dart';

sealed class PushEvent extends Equatable {
  const PushEvent();

  @override
  List<Object?> get props => [];
}

/// Dispatched once, at app start (`app.dart`, alongside `AuthAppStarted`)
/// — sets up the message/token-refresh listeners and requests
/// notification permission. Does not itself register a device token;
/// that only happens once a session exists (`PushRegistrationRequested`).
class PushInitialized extends PushEvent {
  const PushInitialized();
}

/// Dispatched by a listener watching `AuthBloc` reach
/// `AuthAuthenticated` (never from inside `PushBloc` itself reacting to
/// `AuthBloc` directly — `push` doesn't import `auth`'s bloc). Also
/// re-dispatched internally on `onTokenRefresh`.
class PushRegistrationRequested extends PushEvent {
  const PushRegistrationRequested();
}

class PushForegroundBannerDismissed extends PushEvent {
  const PushForegroundBannerDismissed();
}

/// The UI already navigated to `pendingLeadId` — clears it so the same
/// deeplink doesn't fire again on an unrelated rebuild.
class PushDeeplinkConsumed extends PushEvent {
  const PushDeeplinkConsumed();
}

/// Internal — bridges `ObservePushMessagesUseCase.onForegroundMessage`'s
/// stream into the bloc's own event queue (the standard `bloc` pattern:
/// a stream listener never calls `emit` directly, it dispatches an event
/// that a registered handler processes). Never dispatched by UI code.
class PushForegroundMessageArrived extends PushEvent {
  final PushMessage message;

  const PushForegroundMessageArrived(this.message);

  @override
  List<Object?> get props => [message];
}

/// Internal, same reasoning as [PushForegroundMessageArrived] — covers
/// BOTH `onMessageOpenedApp` (background, tapped) and the cold-start
/// `getInitialMessage()` result (app was closed, tapped): TD §10 treats
/// them identically, both just set `pendingLeadId`.
class PushMessageTapped extends PushEvent {
  final PushMessage message;

  const PushMessageTapped(this.message);

  @override
  List<Object?> get props => [message];
}
