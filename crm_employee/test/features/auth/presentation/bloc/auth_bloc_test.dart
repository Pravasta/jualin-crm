// Bloc-level proof of TD §4.1's whole app-open state machine — mocks the
// six use cases (mocktail), never touches the network or a real device.
// Complements test/api_client_test.dart, which proves the lower-level
// HTTP/refresh behavior these use cases sit on top of.
import 'package:bloc_test/bloc_test.dart';
import 'package:crm_employee/core/error/failures.dart';
import 'package:crm_employee/core/usecases/usecase.dart';
import 'package:crm_employee/features/auth/domain/entities/auth_user.dart';
import 'package:crm_employee/features/auth/domain/usecases/authenticate_with_biometrics_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/check_biometric_availability_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/check_stored_session_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/get_current_user_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/login_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/logout_usecase.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_bloc.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_event.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_state.dart';
import 'package:dartz/dartz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockCheckStoredSessionUseCase extends Mock
    implements CheckStoredSessionUseCase {}

class MockCheckBiometricAvailabilityUseCase extends Mock
    implements CheckBiometricAvailabilityUseCase {}

class MockAuthenticateWithBiometricsUseCase extends Mock
    implements AuthenticateWithBiometricsUseCase {}

class MockLoginUseCase extends Mock implements LoginUseCase {}

class MockLogoutUseCase extends Mock implements LogoutUseCase {}

class MockGetCurrentUserUseCase extends Mock implements GetCurrentUserUseCase {}

const tUser = AuthUser(
  userId: 'u1',
  email: 'employee@example.com',
  fullName: 'Employee Satu',
  organizationId: 'o1',
  organizationName: 'Toko Satu',
  membershipId: 'm1',
  role: 'employee',
);

void main() {
  late MockCheckStoredSessionUseCase checkStoredSession;
  late MockCheckBiometricAvailabilityUseCase checkBiometricAvailability;
  late MockAuthenticateWithBiometricsUseCase authenticateWithBiometrics;
  late MockLoginUseCase login;
  late MockLogoutUseCase logout;
  late MockGetCurrentUserUseCase getCurrentUser;

  setUpAll(() {
    registerFallbackValue(const NoParams());
    registerFallbackValue(const LoginParams(email: '', password: ''));
  });

  setUp(() {
    checkStoredSession = MockCheckStoredSessionUseCase();
    checkBiometricAvailability = MockCheckBiometricAvailabilityUseCase();
    authenticateWithBiometrics = MockAuthenticateWithBiometricsUseCase();
    login = MockLoginUseCase();
    logout = MockLogoutUseCase();
    getCurrentUser = MockGetCurrentUserUseCase();
  });

  AuthBloc buildBloc() => AuthBloc(
    checkStoredSession: checkStoredSession,
    checkBiometricAvailability: checkBiometricAvailability,
    authenticateWithBiometrics: authenticateWithBiometrics,
    login: login,
    logout: logout,
    getCurrentUser: getCurrentUser,
  );

  blocTest<AuthBloc, AuthState>(
    'AuthAppStarted with no stored session goes straight to AuthNeedsPassword',
    setUp: () {
      when(() => checkStoredSession()).thenAnswer((_) async => false);
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const AuthAppStarted()),
    expect: () => [const AuthChecking(), const AuthNeedsPassword()],
    verify: (_) {
      verifyNever(() => checkBiometricAvailability());
    },
  );

  blocTest<AuthBloc, AuthState>(
    'a stored session with no biometric enrolled falls through to AuthNeedsPassword explicitly, never skipped',
    setUp: () {
      when(() => checkStoredSession()).thenAnswer((_) async => true);
      when(() => checkBiometricAvailability()).thenAnswer((_) async => false);
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const AuthAppStarted()),
    expect: () => [const AuthChecking(), const AuthNeedsPassword()],
    verify: (_) {
      verifyNever(() => authenticateWithBiometrics());
      verifyNever(() => getCurrentUser(const NoParams()));
    },
  );

  blocTest<AuthBloc, AuthState>(
    'a stored session with biometric available and successful match loads the user',
    setUp: () {
      when(() => checkStoredSession()).thenAnswer((_) async => true);
      when(() => checkBiometricAvailability()).thenAnswer((_) async => true);
      when(() => authenticateWithBiometrics()).thenAnswer((_) async => true);
      when(
        () => getCurrentUser(const NoParams()),
      ).thenAnswer((_) async => const Right(tUser));
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const AuthAppStarted()),
    expect: () => [
      const AuthChecking(),
      const AuthNeedsBiometric(),
      const AuthAuthenticated(tUser),
    ],
  );

  blocTest<AuthBloc, AuthState>(
    'a failed biometric match stays on AuthNeedsBiometric with an error, never lets the user in',
    setUp: () {
      when(() => checkStoredSession()).thenAnswer((_) async => true);
      when(() => checkBiometricAvailability()).thenAnswer((_) async => true);
      when(() => authenticateWithBiometrics()).thenAnswer((_) async => false);
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const AuthAppStarted()),
    expect: () => [
      const AuthChecking(),
      const AuthNeedsBiometric(),
      const AuthNeedsBiometric(
        error: 'Autentikasi biometrik gagal atau dibatalkan.',
      ),
    ],
    verify: (_) {
      verifyNever(() => getCurrentUser(const NoParams()));
    },
  );

  blocTest<AuthBloc, AuthState>(
    'AuthUsePasswordRequested from the biometric gate goes to AuthNeedsPassword',
    build: buildBloc,
    act: (bloc) => bloc.add(const AuthUsePasswordRequested()),
    expect: () => [const AuthNeedsPassword()],
  );

  blocTest<AuthBloc, AuthState>(
    'a successful login loads the user and reaches AuthAuthenticated',
    setUp: () {
      when(
        () => login(const LoginParams(email: 'a@b.com', password: 'right')),
      ).thenAnswer((_) async => const Right(null));
      when(
        () => getCurrentUser(const NoParams()),
      ).thenAnswer((_) async => const Right(tUser));
    },
    build: buildBloc,
    act: (bloc) =>
        bloc.add(const AuthLoginSubmitted(email: 'a@b.com', password: 'right')),
    expect: () => [
      const AuthNeedsPassword(isSubmitting: true),
      const AuthAuthenticated(tUser),
    ],
  );

  blocTest<AuthBloc, AuthState>(
    'a rejected login surfaces the failure message and never calls getCurrentUser',
    setUp: () {
      when(
        () => login(const LoginParams(email: 'a@b.com', password: 'wrong')),
      ).thenAnswer(
        (_) async =>
            const Left(InvalidCredentialsFailure('Email atau password salah.')),
      );
    },
    build: buildBloc,
    act: (bloc) =>
        bloc.add(const AuthLoginSubmitted(email: 'a@b.com', password: 'wrong')),
    expect: () => [
      const AuthNeedsPassword(isSubmitting: true),
      const AuthNeedsPassword(error: 'Email atau password salah.'),
    ],
    verify: (_) {
      verifyNever(() => getCurrentUser(const NoParams()));
    },
  );

  blocTest<AuthBloc, AuthState>(
    'a session that expires right after biometric success returns to AuthNeedsPassword silently (TD §4.2)',
    setUp: () {
      when(() => checkStoredSession()).thenAnswer((_) async => true);
      when(() => checkBiometricAvailability()).thenAnswer((_) async => true);
      when(() => authenticateWithBiometrics()).thenAnswer((_) async => true);
      when(() => getCurrentUser(const NoParams())).thenAnswer(
        (_) async => const Left(SessionExpiredFailure('Sesi Anda berakhir.')),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const AuthAppStarted()),
    expect: () => [
      const AuthChecking(),
      const AuthNeedsBiometric(),
      const AuthNeedsPassword(),
    ],
  );

  blocTest<AuthBloc, AuthState>(
    'AuthLogoutRequested always calls the logout use case and returns to AuthNeedsPassword',
    setUp: () {
      when(
        () => logout(const NoParams()),
      ).thenAnswer((_) async => const Right(null));
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const AuthLogoutRequested()),
    expect: () => [const AuthNeedsPassword()],
    verify: (_) {
      verify(() => logout(const NoParams())).called(1);
    },
  );
}
