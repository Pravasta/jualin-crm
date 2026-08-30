import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../repositories/push_repository.dart';

/// Fetches the current FCM token and registers it in one step — the
/// bloc never handles a raw token itself, only whether this succeeded.
class RegisterDeviceTokenUseCase implements UseCase<void, NoParams> {
  final PushRepository repository;

  const RegisterDeviceTokenUseCase(this.repository);

  @override
  Future<Either<Failure, void>> call(NoParams params) async {
    final token = await repository.getFcmToken();
    if (token == null) {
      return const Left(
        UnexpectedFailure('Token FCM tidak tersedia dari perangkat ini.'),
      );
    }
    return repository.registerToken(token);
  }
}
