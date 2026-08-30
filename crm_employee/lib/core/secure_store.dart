import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// [ApiClient]/[Session] depend on this interface, not on
/// [FlutterSecureStorage] directly — the same "consumer declares the port"
/// shape `crm_be`'s ADR-011 uses for `port.go`, applied here so
/// `api_client_test.dart` can inject an in-memory fake instead of touching
/// a real platform channel (flutter_secure_storage has none in `flutter
/// test`'s host environment).
abstract class TokenStorage {
  Future<void> saveTokens({
    required String accessToken,
    required String refreshToken,
  });
  Future<String?> readAccessToken();
  Future<String?> readRefreshToken();

  /// Removes both tokens — called on logout and on an unrecoverable
  /// refresh failure (TD §5). Never partial: a caller that only cleared
  /// the refresh token would leave a still-readable, soon-to-expire
  /// access token behind.
  Future<void> clear();
}

/// `flutter_secure_storage` — Android Keystore-backed
/// `EncryptedSharedPreferences`. Never `SharedPreferences` directly
/// (acceptance criterion #5): that store is plaintext, world-readable on a
/// rooted device, and a refresh token in it is a 90-day bearer credential
/// for the whole session.
class SecureTokenStorage implements TokenStorage {
  final FlutterSecureStorage _storage;

  const SecureTokenStorage({this._storage = const FlutterSecureStorage()});

  static const _accessTokenKey = 'access_token';
  static const _refreshTokenKey = 'refresh_token';

  @override
  Future<void> saveTokens({
    required String accessToken,
    required String refreshToken,
  }) async {
    await _storage.write(key: _accessTokenKey, value: accessToken);
    await _storage.write(key: _refreshTokenKey, value: refreshToken);
  }

  @override
  Future<String?> readAccessToken() => _storage.read(key: _accessTokenKey);

  @override
  Future<String?> readRefreshToken() => _storage.read(key: _refreshTokenKey);

  @override
  Future<void> clear() async {
    await _storage.delete(key: _accessTokenKey);
    await _storage.delete(key: _refreshTokenKey);
  }
}
