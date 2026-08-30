import 'package:dartz/dartz.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/network/run_api_call.dart';
import '../../domain/entities/notification.dart';
import '../../domain/repositories/notification_repository.dart';
import '../datasources/notification_remote_data_source.dart';
import '../models/notification_model.dart';

/// No `cachedGet()` here — see `NotificationListResult`'s own doc
/// comment for why (TD §7 doesn't name this endpoint as cacheable).
class NotificationRepositoryImpl implements NotificationRepository {
  final NotificationRemoteDataSource remoteDataSource;

  const NotificationRepositoryImpl({required this.remoteDataSource});

  @override
  Future<Either<Failure, NotificationListResult>> getNotifications() {
    return runApiCall(() async {
      final data = await remoteDataSource.list();
      final notifications = data
          .map(
            (json) => NotificationModel.fromJson(json as Map<String, dynamic>),
          )
          .toList();
      return NotificationListResult(notifications: notifications);
    });
  }

  @override
  Future<Either<Failure, void>> markRead(String id) {
    return runApiCall(() => remoteDataSource.markRead(id));
  }
}
