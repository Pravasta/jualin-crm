import '../entities/push_message.dart';
import '../repositories/push_repository.dart';

/// Bundles the stream/one-shot exposures `PushBloc` listens to at
/// startup — kept as ONE use case rather than four, since these are
/// plumbing (forwarding a stream getter), not individually distinct
/// business actions the way `RegisterDeviceTokenUseCase` is. The
/// alternative — `PushBloc` depending on `PushRepository` directly for
/// just the streams — would break the one rule every bloc in this app
/// follows without exception: never see a repository, only use cases.
class ObservePushMessagesUseCase {
  final PushRepository repository;

  const ObservePushMessagesUseCase(this.repository);

  Stream<PushMessage> get onForegroundMessage =>
      repository.onForegroundMessage;

  Stream<PushMessage> get onMessageOpenedApp => repository.onMessageOpenedApp;

  Stream<String> get onTokenRefresh => repository.onTokenRefresh;

  Future<PushMessage?> getInitialMessage() => repository.getInitialMessage();
}
