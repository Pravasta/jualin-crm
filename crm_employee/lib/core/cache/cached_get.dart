import 'dart:convert';

import '../api_error.dart';
import 'response_cache.dart';

/// Result of [cachedGet] — [fromCache] and [fetchedAt] are what a screen
/// needs to render the "Data terakhir diperbarui" banner (TD §7); a
/// fresh network response has `fromCache: false` and `fetchedAt: null`
/// (its freshness needs no timestamp — it just happened).
class CachedResult<T> {
  final T data;
  final bool fromCache;
  final DateTime? fetchedAt;

  const CachedResult({
    required this.data,
    required this.fromCache,
    this.fetchedAt,
  });
}

/// Try network → succeeds → cache the raw body & return it. Network
/// unreachable → fall back to whatever is cached under [key] → return it
/// marked `fromCache: true`. Nothing cached either → the network
/// exception propagates, same as an uncached call would.
///
/// Only for the four endpoints TD §7 names as cacheable (`GET /v1/leads`,
/// `/v1/tasks`, `/v1/leads/{id}`, `/v1/leads/{id}/activities`) — callers
/// decide that by choosing to call this at all, not something enforced
/// here.
///
/// Deliberately does NOT catch [ApiError] or [SessionExpiredException] —
/// those mean the server was reached and answered (a real 4xx/5xx, or a
/// session that needs re-authenticating), which is a different situation
/// from "the network is unreachable" and must never be silently masked
/// behind stale cached data. Only [fetch] throwing something else
/// (`SocketException`, connection timeouts, DNS failures — genuine
/// connectivity failure) triggers the cache fallback.
Future<CachedResult<Map<String, dynamic>>> cachedGet({
  required ResponseCache cache,
  required String key,
  required Future<Map<String, dynamic>> Function() fetch,
}) async {
  try {
    final data = await fetch();
    await cache.put(key, jsonEncode(data));
    return CachedResult(data: data, fromCache: false);
  } on ApiError {
    rethrow;
  } on SessionExpiredException {
    rethrow;
  } catch (_) {
    final cached = await cache.get(key);
    if (cached == null) rethrow;
    return CachedResult(
      data: jsonDecode(cached.body) as Map<String, dynamic>,
      fromCache: true,
      fetchedAt: cached.fetchedAt,
    );
  }
}
