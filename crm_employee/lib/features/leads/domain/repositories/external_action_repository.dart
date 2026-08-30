/// Design brief §8.3: "aktivitas hanya dicatat bila aplikasi eksternal
/// benar-benar terbuka — desain tidak boleh menjanjikan 'tercatat'
/// sebelum itu pasti." Both methods return whether the OS successfully
/// handed off to a dialer/WhatsApp — the strongest signal a Flutter app
/// can ever get; neither this app nor any other can observe what
/// happens INSIDE that external app afterward (a call actually
/// connecting, or being hung up seconds later) without OS-level
/// permissions far outside this feature's scope. "Menekan lalu
/// membatalkan" (acceptance criterion) is read as canceling the OS's own
/// app-picker/permission prompt BEFORE handoff — the one cancellation
/// this layer can actually detect.
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
}
