import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// The FCM token this device is currently registered with the backend
/// under — read by `auth` (to unregister it on logout, while the access
/// token is still valid) and written by `push` (after a successful
/// `POST /v1/device-tokens`), without either feature importing the
/// other's `features/` tree. Same tier as `TokenStorage` — a `core/`
/// primitive both depend on, the same reasoning `ResponseCache` is
/// shared between `leads` and `activities` rather than owned by either.
///
/// Not a secret in the sense Aturan #20/#26 mean (an FCM token isn't a
/// bearer credential for the session — knowing it lets someone send
/// pushes to this device via a backend that already checks
/// authorization, nothing more), but `flutter_secure_storage` is reused
/// anyway: it's already a dependency, and there's no reason to add a
/// second storage mechanism for one string.
abstract class PushTokenStore {
  Future<void> save(String token);
  Future<String?> read();
  Future<void> clear();
}

class SecurePushTokenStore implements PushTokenStore {
  final FlutterSecureStorage _storage;

  // NOT `{this._storage = ...}` — see secure_store.dart's
  // SecureTokenStorage constructor for why (a named parameter can't
  // start with an underscore under every Dart SDK, even though this
  // project's own FVM-pinned one doesn't flag it; issue #80).
  const SecurePushTokenStore({
    FlutterSecureStorage storage = const FlutterSecureStorage(),
  })
    // ignore: prefer_initializing_formals
    : _storage = storage;

  static const _key = 'fcm_token';

  @override
  Future<void> save(String token) => _storage.write(key: _key, value: token);

  @override
  Future<String?> read() => _storage.read(key: _key);

  @override
  Future<void> clear() => _storage.delete(key: _key);
}
