/// One row of `response_cache` — TD phase 5 §7's key-value cache. `body`
/// is the raw JSON `crm_be` returned, untouched; `fetchedAt` is what the
/// "Data terakhir diperbarui `<waktu>`" banner reads.
class CachedResponse {
  final String body;
  final DateTime fetchedAt;

  const CachedResponse({required this.body, required this.fetchedAt});
}

/// `LeadRepositoryImpl`/(later, #73) `TaskRepositoryImpl` depend on this
/// interface, not `SqfliteResponseCache` directly — `sqflite` has no
/// real platform channel in `flutter test`'s host environment, same
/// reasoning as `TokenStorage` (`core/secure_store.dart`).
abstract class ResponseCache {
  /// `key` is the request this response answers — TD §7's own example:
  /// `"GET /v1/leads?status=new&page=1"`. Null when nothing was ever
  /// cached for this exact key.
  Future<CachedResponse?> get(String key);

  Future<void> put(String key, String body);

  /// Every row, unconditionally — called on logout (TD §7: "perangkat
  /// yang berpindah pengguna tidak boleh menampilkan sisa data pengguna
  /// sebelumnya"). Never a per-key delete; there's no scenario in this
  /// app where only some cached responses should survive a logout.
  Future<void> clear();
}
