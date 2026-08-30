import '../../domain/repositories/external_action_repository.dart';
import '../datasources/external_app_data_source.dart';

class ExternalActionRepositoryImpl implements ExternalActionRepository {
  final ExternalAppDataSource dataSource;

  const ExternalActionRepositoryImpl(this.dataSource);

  @override
  Future<bool> launchDialer(String phone) {
    return dataSource.launch(Uri(scheme: 'tel', path: phone));
  }

  @override
  Future<bool> launchWhatsApp(String phoneE164) {
    // wa.me needs digits only — no leading '+' (crm_be's phone_e164
    // always includes one).
    final digits = phoneE164.replaceAll('+', '');
    return dataSource.launch(Uri.parse('https://wa.me/$digits'));
  }
}
