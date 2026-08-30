// Proves AuthRepositoryImpl's boundary-mapping logic — ApiError /
// SessionExpiredException (core/api_error.dart, network vocabulary)
// translated into Failure (core/error/failures.dart, domain vocabulary)
// — and its orchestration of AuthRemoteDataSource + TokenStorage
// together, which neither one alone can prove.
import 'package:crm_employee/core/api_error.dart';
import 'package:crm_employee/core/error/failures.dart';
import 'package:crm_employee/core/secure_store.dart';
import 'package:crm_employee/features/auth/data/datasources/auth_remote_data_source.dart';
import 'package:crm_employee/features/auth/data/models/auth_user_model.dart';
import 'package:crm_employee/features/auth/data/repositories/auth_repository_impl.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockAuthRemoteDataSource extends Mock implements AuthRemoteDataSource {}

class MockTokenStorage extends Mock implements TokenStorage {}

void main() {
  late MockAuthRemoteDataSource remoteDataSource;
  late MockTokenStorage tokenStorage;
  late AuthRepositoryImpl repository;

  setUp(() {
    remoteDataSource = MockAuthRemoteDataSource();
    tokenStorage = MockTokenStorage();
    repository = AuthRepositoryImpl(
      remoteDataSource: remoteDataSource,
      tokenStorage: tokenStorage,
    );
    registerFallbackValue(<String, dynamic>{});
  });

  group('hasStoredSession', () {
    test('true when a refresh token is stored', () async {
      when(
        () => tokenStorage.readRefreshToken(),
      ).thenAnswer((_) async => 'a-refresh-token');
      expect(await repository.hasStoredSession(), isTrue);
    });

    test('false when nothing is stored', () async {
      when(() => tokenStorage.readRefreshToken()).thenAnswer((_) async => null);
      expect(await repository.hasStoredSession(), isFalse);
    });
  });

  group('login', () {
    test('stores tokens and returns Right on success', () async {
      when(
        () => remoteDataSource.login(email: 'a@b.com', password: 'right'),
      ).thenAnswer(
        (_) async => {'access_token': 'access-1', 'refresh_token': 'refresh-1'},
      );
      when(
        () => tokenStorage.saveTokens(
          accessToken: any(named: 'accessToken'),
          refreshToken: any(named: 'refreshToken'),
        ),
      ).thenAnswer((_) async {});

      final result = await repository.login(
        email: 'a@b.com',
        password: 'right',
      );

      expect(result.isRight(), isTrue);
      verify(
        () => tokenStorage.saveTokens(
          accessToken: 'access-1',
          refreshToken: 'refresh-1',
        ),
      ).called(1);
    });

    test(
      'maps ApiError(invalid_credentials) to InvalidCredentialsFailure',
      () async {
        when(
          () => remoteDataSource.login(email: 'a@b.com', password: 'wrong'),
        ).thenThrow(
          const ApiError(
            status: 401,
            code: 'invalid_credentials',
            message: 'Email atau password salah.',
          ),
        );

        final result = await repository.login(
          email: 'a@b.com',
          password: 'wrong',
        );

        expect(
          result.fold((f) => f, (_) => null),
          isA<InvalidCredentialsFailure>().having(
            (f) => f.message,
            'message',
            'Email atau password salah.',
          ),
        );
        verifyNever(
          () => tokenStorage.saveTokens(
            accessToken: any(named: 'accessToken'),
            refreshToken: any(named: 'refreshToken'),
          ),
        );
      },
    );

    test(
      'maps any other ApiError to UnexpectedFailure, not InvalidCredentialsFailure',
      () async {
        when(
          () => remoteDataSource.login(email: 'a@b.com', password: 'x'),
        ).thenThrow(
          const ApiError(
            status: 429,
            code: 'rate_limited',
            message: 'Terlalu banyak percobaan.',
          ),
        );

        final result = await repository.login(email: 'a@b.com', password: 'x');

        expect(result.fold((f) => f, (_) => null), isA<UnexpectedFailure>());
      },
    );
  });

  group('getCurrentUser', () {
    const model = AuthUserModel(
      userId: 'u1',
      email: 'employee@example.com',
      fullName: 'Employee Satu',
      organizationId: 'o1',
      organizationName: 'Toko Satu',
      membershipId: 'm1',
      role: 'employee',
    );

    test('returns Right(AuthUser) on success', () async {
      when(
        () => remoteDataSource.getCurrentUser(),
      ).thenAnswer((_) async => model);

      final result = await repository.getCurrentUser();

      expect(result.fold((_) => null, (u) => u), equals(model));
    });

    test('maps SessionExpiredException to SessionExpiredFailure', () async {
      when(
        () => remoteDataSource.getCurrentUser(),
      ).thenThrow(const SessionExpiredException());

      final result = await repository.getCurrentUser();

      expect(result.fold((f) => f, (_) => null), isA<SessionExpiredFailure>());
    });
  });

  group('logout', () {
    test(
      'clears local tokens and returns Right even when the backend call fails',
      () async {
        when(
          () => tokenStorage.readRefreshToken(),
        ).thenAnswer((_) async => 'a-refresh-token');
        when(
          () =>
              remoteDataSource.logout(refreshToken: any(named: 'refreshToken')),
        ).thenThrow(Exception('network down'));
        when(() => tokenStorage.clear()).thenAnswer((_) async {});

        final result = await repository.logout();

        expect(result.isRight(), isTrue);
        verify(() => tokenStorage.clear()).called(1);
      },
    );
  });
}
