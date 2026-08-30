import '../../../../core/api_client.dart';

abstract class NotificationRemoteDataSource {
  /// Returns the raw list, already narrowed to `data` — like
  /// `ActivityRemoteDataSource.listActivities`, this envelope has no
  /// `meta` (Aturan #33's plain-list form).
  Future<List<dynamic>> list();

  Future<void> markRead(String id);
}

class NotificationRemoteDataSourceImpl implements NotificationRemoteDataSource {
  final ApiClient client;

  const NotificationRemoteDataSourceImpl(this.client);

  @override
  Future<List<dynamic>> list() async {
    final data = await client.send('/v1/notifications');
    return data as List<dynamic>;
  }

  @override
  Future<void> markRead(String id) async {
    await client.send('/v1/notifications/$id/read', method: 'POST');
  }
}
