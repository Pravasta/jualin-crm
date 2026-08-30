// Proves the URI ExternalActionRepositoryImpl builds — not url_launcher
// itself (ExternalAppDataSource is mocked, same reasoning
// BiometricLocalDataSource's tests use to avoid a real platform channel).
import 'package:crm_employee/features/leads/data/datasources/external_app_data_source.dart';
import 'package:crm_employee/features/leads/data/repositories/external_action_repository_impl.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockExternalAppDataSource extends Mock implements ExternalAppDataSource {}

void main() {
  late MockExternalAppDataSource dataSource;
  late ExternalActionRepositoryImpl repository;

  setUpAll(() {
    registerFallbackValue(Uri.parse('https://example.com'));
  });

  setUp(() {
    dataSource = MockExternalAppDataSource();
    repository = ExternalActionRepositoryImpl(dataSource);
  });

  group('launchDialer', () {
    test('builds a tel: URI from the phone exactly as stored', () async {
      when(() => dataSource.launch(any())).thenAnswer((_) async => true);

      final result = await repository.launchDialer('0812-3456-7890');

      expect(result, isTrue);
      final uri = verify(() => dataSource.launch(captureAny())).captured.single as Uri;
      expect(uri.scheme, 'tel');
      expect(uri.path, '0812-3456-7890');
    });

    test('propagates false when the OS never actually launched anything', () async {
      when(() => dataSource.launch(any())).thenAnswer((_) async => false);

      final result = await repository.launchDialer('0812');

      expect(result, isFalse);
    });
  });

  group('launchWhatsApp', () {
    test('builds a wa.me URI stripping the leading + from phoneE164', () async {
      when(() => dataSource.launch(any())).thenAnswer((_) async => true);

      final result = await repository.launchWhatsApp('+6281234567890');

      expect(result, isTrue);
      final uri = verify(() => dataSource.launch(captureAny())).captured.single as Uri;
      expect(uri.toString(), 'https://wa.me/6281234567890');
    });

    test('propagates false when the OS never actually launched anything', () async {
      when(() => dataSource.launch(any())).thenAnswer((_) async => false);

      final result = await repository.launchWhatsApp('+6281234567890');

      expect(result, isFalse);
    });
  });
}
