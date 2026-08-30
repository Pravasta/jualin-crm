import '../api_client.dart';

/// `POST`/`DELETE /v1/device-tokens` — deliberately in `core/`, not
/// `features/push/`: `auth`'s logout flow needs [unregister] too (the
/// access token is still valid at that point, before `LogoutUseCase`
/// clears it — waiting until `push` could react would be too late), and
/// `core/` never imports `features/` in either direction. `features/
/// push/`'s own repository is what orchestrates the rest of the FCM
/// lifecycle (getting tokens, listening to streams, permission) — this
/// class only knows how to talk to these two endpoints.
abstract class DeviceTokenRemoteDataSource {
  Future<void> register({required String token, required String platform});

  Future<void> unregister(String token);
}

class DeviceTokenRemoteDataSourceImpl implements DeviceTokenRemoteDataSource {
  final ApiClient client;

  const DeviceTokenRemoteDataSourceImpl(this.client);

  @override
  Future<void> register({
    required String token,
    required String platform,
  }) async {
    await client.send(
      '/v1/device-tokens',
      method: 'POST',
      body: {'token': token, 'platform': platform},
    );
  }

  @override
  Future<void> unregister(String token) async {
    // 204 No Content — client.send() already handles a body-less
    // response, nothing to parse here.
    await client.send(
      '/v1/device-tokens',
      method: 'DELETE',
      body: {'token': token},
    );
  }
}
