import 'dart:async';

import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/usecases/usecase.dart';
import '../../domain/usecases/observe_push_messages_usecase.dart';
import '../../domain/usecases/register_device_token_usecase.dart';
import '../../domain/usecases/request_notification_permission_usecase.dart';
import 'push_event.dart';
import 'push_state.dart';

class PushBloc extends Bloc<PushEvent, PushState> {
  final RequestNotificationPermissionUseCase requestPermission;
  final RegisterDeviceTokenUseCase registerDeviceToken;
  final ObservePushMessagesUseCase observeMessages;

  StreamSubscription<void>? _foregroundSub;
  StreamSubscription<void>? _tappedSub;
  StreamSubscription<void>? _tokenRefreshSub;

  PushBloc({
    required this.requestPermission,
    required this.registerDeviceToken,
    required this.observeMessages,
  }) : super(const PushState()) {
    on<PushInitialized>(_onInitialized);
    on<PushRegistrationRequested>(_onRegistrationRequested);
    on<PushForegroundBannerDismissed>(_onBannerDismissed);
    on<PushDeeplinkConsumed>(_onDeeplinkConsumed);
    on<PushForegroundMessageArrived>(_onForegroundMessageArrived);
    on<PushMessageTapped>(_onMessageTapped);
  }

  Future<void> _onInitialized(
    PushInitialized event,
    Emitter<PushState> emit,
  ) async {
    // Best-effort — a denied permission just means the OS won't show a
    // tray notification for pushes that arrive later; the app still
    // works, nothing here needs to react to the result.
    await requestPermission();

    // Cold start: the app may have been launched BY a notification tap.
    // getInitialMessage() only ever answers this once, right here — not
    // a stream, so this can't be re-checked later.
    final initial = await observeMessages.getInitialMessage();
    if (initial != null) add(PushMessageTapped(initial));

    _foregroundSub = observeMessages.onForegroundMessage.listen(
      (message) => add(PushForegroundMessageArrived(message)),
    );
    _tappedSub = observeMessages.onMessageOpenedApp.listen(
      (message) => add(PushMessageTapped(message)),
    );
    // A rotated token is only useful once re-registered — see
    // PushRepository.onTokenRefresh's doc comment.
    _tokenRefreshSub = observeMessages.onTokenRefresh.listen(
      (_) => add(const PushRegistrationRequested()),
    );
  }

  Future<void> _onRegistrationRequested(
    PushRegistrationRequested event,
    Emitter<PushState> emit,
  ) async {
    // Best-effort — same reasoning as requestPermission above. A failed
    // registration means this device won't get push until the next
    // successful attempt (next login, next token refresh); never worth
    // surfacing as a user-facing error over what is, from the user's
    // point of view, a background concern.
    await registerDeviceToken(const NoParams());
  }

  void _onBannerDismissed(
    PushForegroundBannerDismissed event,
    Emitter<PushState> emit,
  ) {
    emit(PushState(pendingLeadId: state.pendingLeadId));
  }

  void _onDeeplinkConsumed(
    PushDeeplinkConsumed event,
    Emitter<PushState> emit,
  ) {
    emit(PushState(foregroundMessage: state.foregroundMessage));
  }

  void _onForegroundMessageArrived(
    PushForegroundMessageArrived event,
    Emitter<PushState> emit,
  ) {
    emit(
      PushState(
        pendingLeadId: state.pendingLeadId,
        foregroundMessage: event.message,
      ),
    );
  }

  void _onMessageTapped(PushMessageTapped event, Emitter<PushState> emit) {
    // No lead_id — nothing to deeplink to (defensive; every message this
    // app currently sends has one, see PushMessage's doc comment).
    if (event.message.leadId == null) return;
    emit(
      PushState(
        pendingLeadId: event.message.leadId,
        foregroundMessage: state.foregroundMessage,
      ),
    );
  }

  @override
  Future<void> close() {
    _foregroundSub?.cancel();
    _tappedSub?.cancel();
    _tokenRefreshSub?.cancel();
    return super.close();
  }
}
