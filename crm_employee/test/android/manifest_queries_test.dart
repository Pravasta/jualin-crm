import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// Issue #147. Since Android 11 url_launcher's canLaunchUrl can only see apps
// the manifest declares under <queries>. The entries for tel and https were
// missing, so "Telepon" and "WhatsApp" did nothing on every real phone — and
// no Dart test could notice, because the manifest is not Dart. This one reads
// the file itself, so deleting an entry turns CI red instead of silently
// breaking the two buttons the mobile app exists for.
void main() {
  final manifest = File('android/app/src/main/AndroidManifest.xml').readAsStringSync();
  final queries = RegExp(r'<queries>([\s\S]*?)</queries>').firstMatch(manifest)?.group(1) ?? '';

  bool declaresViewFor(String scheme) => RegExp(
        r'<intent>\s*<action android:name="android\.intent\.action\.VIEW"\s*/>\s*'
        '<data android:scheme="$scheme"\\s*/>\\s*</intent>',
      ).hasMatch(queries);

  test('<queries> exists at all', () {
    expect(queries, isNotEmpty);
  });

  test('declares tel — the Telepon button hands tel: URIs to url_launcher', () {
    expect(declaresViewFor('tel'), isTrue);
  });

  test('declares https — the WhatsApp button opens https://wa.me/...', () {
    expect(declaresViewFor('https'), isTrue);
  });
}
