/// Design brief §8.3: "aktivitas hanya dicatat bila aplikasi eksternal
/// benar-benar terbuka — desain tidak boleh menjanjikan 'tercatat'
/// sebelum itu pasti." Both methods return whether the OS successfully
/// handed off to a dialer/WhatsApp — the strongest signal a Flutter app
/// can ever get; neither this app nor any other can observe what
/// happens INSIDE that external app afterward (a call actually
/// connecting, or being hung up seconds later) without OS-level
/// permissions far outside this feature's scope.
///
/// `false` means nothing on the device could take the hand-off — never
/// that the user canceled (issue #147). A user backing out of the dialer
/// happens AFTER a successful hand-off and cannot be observed here, so
/// "menekan lalu membatalkan" (acceptance criterion) is satisfied simply
/// because no activity is logged until the hand-off itself succeeds.
abstract class ExternalActionRepository {
  /// `tel:` — always attempted with [phone] exactly as stored (never
  /// [Lead.phoneE164]); a dialer accepts almost any string a human could
  /// type, so there is no format this needs to reject client-side
  /// (acceptance criterion #6, #72).
  Future<bool> launchDialer(String phone);

  /// `https://wa.me/<digits>` — REQUIRES a real international number
  /// with no leading `+`; a malformed one opens WhatsApp to a broken
  /// conversation rather than failing cleanly, which is worse than not
  /// offering the button at all. Callers must only call this when
  /// [Lead.phoneE164] is non-null — this method does not itself defend
  /// against a null/malformed number.
  Future<bool> launchWhatsApp(String phoneE164);

  /// `mailto:` with [email] as stored (issue #149). Unlike the two above,
  /// a successful hand-off is NOT logged as an activity — the activity-type
  /// list is closed and has no email type; adding one is a migration the
  /// product owner chose not to make for a secondary channel.
  Future<bool> launchEmail(String email);
}
