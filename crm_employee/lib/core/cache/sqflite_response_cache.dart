import 'package:sqflite/sqflite.dart';

import 'response_cache.dart';

/// TD phase 5 §7 — deliberately a single key-value table, not a local
/// domain schema (drift/ORM). What acceptance criterion #2 needs is
/// showing back exactly what was last seen, not local query/join/filter
/// — a second schema that has to track every backend change is a real
/// cost for a capability nobody asked for (Aturan #27, #28).
class SqfliteResponseCache implements ResponseCache {
  Database? _db;

  Future<Database> _database() async {
    final existing = _db;
    if (existing != null) return existing;

    final path = await getDatabasesPath();
    final db = await openDatabase(
      '$path/response_cache.db',
      version: 1,
      onCreate: (db, version) {
        return db.execute('''
          CREATE TABLE response_cache (
            key TEXT PRIMARY KEY,
            body TEXT NOT NULL,
            fetched_at INTEGER NOT NULL
          )
        ''');
      },
    );
    _db = db;
    return db;
  }

  @override
  Future<CachedResponse?> get(String key) async {
    final db = await _database();
    final rows = await db.query(
      'response_cache',
      where: 'key = ?',
      whereArgs: [key],
      limit: 1,
    );
    if (rows.isEmpty) return null;
    final row = rows.first;
    return CachedResponse(
      body: row['body'] as String,
      fetchedAt: DateTime.fromMillisecondsSinceEpoch(
        row['fetched_at'] as int,
      ),
    );
  }

  @override
  Future<void> put(String key, String body) async {
    final db = await _database();
    await db.insert('response_cache', {
      'key': key,
      'body': body,
      'fetched_at': DateTime.now().millisecondsSinceEpoch,
    }, conflictAlgorithm: ConflictAlgorithm.replace);
  }

  @override
  Future<void> clear() async {
    final db = await _database();
    await db.delete('response_cache');
  }
}
