import 'package:url_launcher/url_launcher.dart' as url_launcher;

/// Wraps `package:url_launcher`'s free functions — no real platform
/// channel in `flutter test`'s host environment, same reasoning as
/// `BiometricLocalDataSource` (#69). `ExternalActionRepositoryImpl` is
/// what builds the actual `tel:`/`https://wa.me/...` URIs; this layer
/// only knows how to hand a URI to the OS.
abstract class ExternalAppDataSource {
  /// True only if the OS reports it actually launched something —
  /// `canLaunchUrl` failing (no app can handle it) or the launch itself
  /// failing both return false here, never an exception.
  Future<bool> launch(Uri uri);
}

class ExternalAppDataSourceImpl implements ExternalAppDataSource {
  @override
  Future<bool> launch(Uri uri) async {
    final canLaunch = await url_launcher.canLaunchUrl(uri);
    if (!canLaunch) return false;
    return url_launcher.launchUrl(
      uri,
      mode: url_launcher.LaunchMode.externalApplication,
    );
  }
}
