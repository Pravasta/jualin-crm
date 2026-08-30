import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/core/error/failures.dart';
import 'package:crm_employee/core/network/run_api_call.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('a successful call returns Right with its value', () async {
    final result = await runApiCall(() async => 42);
    expect(result.fold((_) => null, (v) => v), 42);
  });

  test('SessionExpiredException maps to SessionExpiredFailure', () async {
    final result = await runApiCall<int>(
      () async => throw const SessionExpiredException(),
    );
    expect(result.fold((f) => f, (_) => null), isA<SessionExpiredFailure>());
  });

  test('an ApiError with no onApiError override maps to UnexpectedFailure', () async {
    final result = await runApiCall<int>(
      () async => throw const ApiError(
        status: 500,
        code: 'internal_error',
        message: 'Terjadi kesalahan internal.',
      ),
    );
    expect(
      result.fold((f) => f, (_) => null),
      isA<UnexpectedFailure>().having(
        (f) => f.message,
        'message',
        'Terjadi kesalahan internal.',
      ),
    );
  });

  test('onApiError lets a caller override the mapping for a specific code', () async {
    final result = await runApiCall<int>(
      () async => throw const ApiError(
        status: 401,
        code: 'invalid_credentials',
        message: 'Email atau password salah.',
      ),
      onApiError: (e) =>
          e.code == 'invalid_credentials'
          ? InvalidCredentialsFailure(e.message)
          : UnexpectedFailure(e.message),
    );
    expect(
      result.fold((f) => f, (_) => null),
      isA<InvalidCredentialsFailure>(),
    );
  });

  test('onApiError still falls through to UnexpectedFailure for codes it does not handle', () async {
    final result = await runApiCall<int>(
      () async => throw const ApiError(
        status: 429,
        code: 'rate_limited',
        message: 'Terlalu banyak percobaan.',
      ),
      onApiError: (e) =>
          e.code == 'invalid_credentials'
          ? InvalidCredentialsFailure(e.message)
          : UnexpectedFailure(e.message),
    );
    expect(result.fold((f) => f, (_) => null), isA<UnexpectedFailure>());
  });

  test('any other exception (connectivity) maps to a generic UnexpectedFailure', () async {
    final result = await runApiCall<int>(
      () async => throw Exception('no connectivity'),
    );
    expect(
      result.fold((f) => f, (_) => null),
      isA<UnexpectedFailure>().having(
        (f) => f.message,
        'message',
        'Tidak dapat terhubung ke server. Periksa koneksi Anda.',
      ),
    );
  });
}
