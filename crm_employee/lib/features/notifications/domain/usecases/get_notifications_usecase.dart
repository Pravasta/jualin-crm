import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/notification.dart';
import '../repositories/notification_repository.dart';

class GetNotificationsUseCase
    implements UseCase<NotificationListResult, NoParams> {
  final NotificationRepository repository;

  const GetNotificationsUseCase(this.repository);

  @override
  Future<Either<Failure, NotificationListResult>> call(NoParams params) {
    return repository.getNotifications();
  }
}
