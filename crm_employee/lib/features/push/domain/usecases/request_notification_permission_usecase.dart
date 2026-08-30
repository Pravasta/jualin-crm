import '../repositories/push_repository.dart';

/// A denied permission isn't a domain `Failure` — it's a valid outcome
/// the OS lets the user choose (same reasoning `LaunchDialerUseCase`
/// isn't `Either`-wrapped, #72).
class RequestNotificationPermissionUseCase {
  final PushRepository repository;

  const RequestNotificationPermissionUseCase(this.repository);

  Future<bool> call() => repository.requestPermission();
}
