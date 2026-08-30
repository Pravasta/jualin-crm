import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../entities/notification.dart';

abstract class NotificationRepository {
  Future<Either<Failure, NotificationListResult>> getNotifications();

  /// `POST /v1/notifications/{id}/read` — fire-and-forget from the UI's
  /// point of view (tapping a row marks it read AND navigates in the
  /// same gesture); a failure here is never worth blocking or
  /// interrupting the navigation over.
  Future<Either<Failure, void>> markRead(String id);
}
