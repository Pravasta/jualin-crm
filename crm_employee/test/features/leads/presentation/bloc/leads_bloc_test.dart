import 'package:bloc_test/bloc_test.dart';
import 'package:crm_employee/core/error/failures.dart';
import 'package:crm_employee/features/auth/domain/usecases/authenticate_with_biometrics_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/check_biometric_availability_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/check_stored_session_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/get_current_user_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/login_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/logout_usecase.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_bloc.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_state.dart';
import 'package:crm_employee/features/leads/domain/entities/lead.dart';
import 'package:crm_employee/features/leads/domain/usecases/get_my_leads_usecase.dart';
import 'package:crm_employee/features/leads/presentation/bloc/leads_bloc.dart';
import 'package:crm_employee/features/leads/presentation/bloc/leads_event.dart';
import 'package:crm_employee/features/leads/presentation/bloc/leads_state.dart';
import 'package:dartz/dartz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockGetMyLeadsUseCase extends Mock implements GetMyLeadsUseCase {}

// AuthBloc's own use cases are irrelevant here — LeadsBloc only ever
// calls authBloc.add(...), it never inspects AuthBloc's state. A real
// AuthBloc wired to mocked (never-called) use cases is simpler than
// mocking AuthBloc itself, which bloc_test/mocktail can't easily stand
// in for since AuthBloc's `add` isn't a plain method call to verify in
// isolation from its internal event loop.
class MockCheckStoredSessionUseCase extends Mock
    implements CheckStoredSessionUseCase {}

class MockCheckBiometricAvailabilityUseCase extends Mock
    implements CheckBiometricAvailabilityUseCase {}

class MockAuthenticateWithBiometricsUseCase extends Mock
    implements AuthenticateWithBiometricsUseCase {}

class MockLoginUseCase extends Mock implements LoginUseCase {}

class MockLogoutUseCase extends Mock implements LogoutUseCase {}

class MockGetCurrentUserUseCase extends Mock implements GetCurrentUserUseCase {}

Lead _lead(String id, String status) => Lead(
  id: id,
  leadNumber: 1,
  name: 'Rina Wijaya',
  status: status,
  source: 'manual',
  version: 1,
  createdAt: DateTime(2026, 8, 30),
  updatedAt: DateTime(2026, 8, 30),
);

void main() {
  late MockGetMyLeadsUseCase getMyLeads;
  late AuthBloc authBloc;

  setUpAll(() {
    registerFallbackValue(const GetMyLeadsParams());
  });

  setUp(() {
    getMyLeads = MockGetMyLeadsUseCase();
    authBloc = AuthBloc(
      checkStoredSession: MockCheckStoredSessionUseCase(),
      checkBiometricAvailability: MockCheckBiometricAvailabilityUseCase(),
      authenticateWithBiometrics: MockAuthenticateWithBiometricsUseCase(),
      login: MockLoginUseCase(),
      logout: MockLogoutUseCase(),
      getCurrentUser: MockGetCurrentUserUseCase(),
    );
  });

  tearDown(() => authBloc.close());

  LeadsBloc buildBloc() => LeadsBloc(getMyLeads: getMyLeads, authBloc: authBloc);

  blocTest<LeadsBloc, LeadsState>(
    'LeadsRequested loads with no filter and reaches LeadsLoaded',
    setUp: () {
      when(() => getMyLeads(any())).thenAnswer(
        (_) async => Right(
          LeadListResult(
            leads: [_lead('1', 'new')],
            total: 1,
            fromCache: false,
          ),
        ),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const LeadsRequested()),
    expect: () => [
      const LeadsLoading(),
      isA<LeadsLoaded>()
          .having((s) => s.leads, 'leads', hasLength(1))
          .having((s) => s.total, 'total', 1)
          .having((s) => s.fromCache, 'fromCache', isFalse),
    ],
    verify: (_) {
      verify(
        () => getMyLeads(const GetMyLeadsParams(status: null, query: null)),
      ).called(1);
    },
  );

  blocTest<LeadsBloc, LeadsState>(
    'LeadsStatusFilterChanged carries the new filter into the next load',
    setUp: () {
      when(() => getMyLeads(any())).thenAnswer(
        (_) async => const Right(
          LeadListResult(leads: [], total: 0, fromCache: false),
        ),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const LeadsStatusFilterChanged('won')),
    expect: () => [
      const LeadsLoading(statusFilter: 'won'),
      const LeadsLoaded(
        leads: [],
        total: 0,
        fromCache: false,
        statusFilter: 'won',
      ),
    ],
    verify: (_) {
      verify(
        () => getMyLeads(const GetMyLeadsParams(status: 'won', query: null)),
      ).called(1);
    },
  );

  blocTest<LeadsBloc, LeadsState>(
    'LeadsSearchChanged carries the query into the next load',
    setUp: () {
      when(() => getMyLeads(any())).thenAnswer(
        (_) async => const Right(
          LeadListResult(leads: [], total: 0, fromCache: false),
        ),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const LeadsSearchChanged('rina')),
    expect: () => [
      const LeadsLoading(query: 'rina'),
      const LeadsLoaded(leads: [], total: 0, fromCache: false, query: 'rina'),
    ],
    verify: (_) {
      verify(
        () => getMyLeads(const GetMyLeadsParams(status: null, query: 'rina')),
      ).called(1);
    },
  );

  blocTest<LeadsBloc, LeadsState>(
    'a cached (offline) result is reflected in LeadsLoaded.fromCache/fetchedAt',
    setUp: () {
      final fetchedAt = DateTime(2026, 8, 30, 8, 14);
      when(() => getMyLeads(any())).thenAnswer(
        (_) async => Right(
          LeadListResult(
            leads: [_lead('1', 'new')],
            total: 1,
            fromCache: true,
            fetchedAt: fetchedAt,
          ),
        ),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const LeadsRequested()),
    expect: () => [
      const LeadsLoading(),
      isA<LeadsLoaded>()
          .having((s) => s.fromCache, 'fromCache', isTrue)
          .having(
            (s) => s.fetchedAt,
            'fetchedAt',
            DateTime(2026, 8, 30, 8, 14),
          ),
    ],
  );

  blocTest<LeadsBloc, LeadsState>(
    'a non-session failure emits LeadsError with the failure message',
    setUp: () {
      when(
        () => getMyLeads(any()),
      ).thenAnswer((_) async => const Left(UnexpectedFailure('Tidak dapat terhubung ke server. Periksa koneksi Anda.')));
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const LeadsRequested()),
    expect: () => [
      const LeadsLoading(),
      const LeadsError('Tidak dapat terhubung ke server. Periksa koneksi Anda.'),
    ],
  );

  blocTest<LeadsBloc, LeadsState>(
    'SessionExpiredFailure dispatches AuthSessionInvalidated to AuthBloc instead of emitting LeadsError',
    setUp: () {
      when(() => getMyLeads(any())).thenAnswer(
        (_) async => const Left(SessionExpiredFailure('Sesi Anda berakhir.')),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const LeadsRequested()),
    // Only LeadsLoading — no LeadsError, the session-expiry case hands
    // off to AuthBloc instead of showing its own error UI.
    expect: () => [const LeadsLoading()],
  );

  test('SessionExpiredFailure actually reaches AuthBloc and flips its state to AuthSessionExpired', () async {
    when(() => getMyLeads(any())).thenAnswer(
      (_) async => const Left(SessionExpiredFailure('Sesi Anda berakhir.')),
    );
    final bloc = buildBloc();

    bloc.add(const LeadsRequested());
    await Future<void>.delayed(const Duration(milliseconds: 50));

    expect(authBloc.state, isA<AuthSessionExpired>());
    await bloc.close();
  });
}
