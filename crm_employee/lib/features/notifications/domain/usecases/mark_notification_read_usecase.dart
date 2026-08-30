import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../repositories/notification_repository.dart';

class MarkNotificationReadUseCase implements UseCase<void, String> {
  final NotificationRepository repository;

  const MarkNotificationReadUseCase(this.repository);

  @override
  Future<Either<Failure, void>> call(String id) {
    return repository.markRead(id);
  }
}
