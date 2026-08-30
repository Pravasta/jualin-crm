/// Design brief §6/§11: relative time ("2j lalu", "kemarin") for
/// "kapan lead terakhir disentuh" — comparing urgency across a list is
/// easier relative than absolute. [absoluteTime] is the opposite
/// deliberately, for the cache banner ("diperbarui 08:14") — TD §7,
/// design brief §11: honesty about the data's age needs a real clock
/// reading, not "beberapa jam lalu" that keeps meaning something
/// different every time the screen renders.
///
/// [now] is injectable so tests don't depend on the wall clock.
String relativeTime(DateTime from, {DateTime? now}) {
  final reference = now ?? DateTime.now();
  final diff = reference.difference(from);

  if (diff.inSeconds < 60) return 'Baru saja';
  if (diff.inMinutes < 60) return '${diff.inMinutes}mnt lalu';
  if (diff.inHours < 24) return '${diff.inHours}j lalu';
  if (diff.inDays == 1) return 'kemarin';
  if (diff.inDays < 30) return '${diff.inDays} hari lalu';
  if (diff.inDays < 365) return '${(diff.inDays / 30).floor()} bulan lalu';
  return '${(diff.inDays / 365).floor()} tahun lalu';
}

/// `HH:mm`, local time — the cache banner's "diperbarui 08:14".
String absoluteTime(DateTime at) {
  final local = at.toLocal();
  final hh = local.hour.toString().padLeft(2, '0');
  final mm = local.minute.toString().padLeft(2, '0');
  return '$hh:$mm';
}
