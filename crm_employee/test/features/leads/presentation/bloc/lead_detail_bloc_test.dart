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
import 'package:crm_employee/features/leads/domain/entities/activity.dart';
import 'package:crm_employee/features/leads/domain/entities/lead.dart';
import 'package:crm_employee/features/leads/domain/usecases/add_lead_note_usecase.dart';
import 'package:crm_employee/features/leads/domain/usecases/get_lead_activities_usecase.dart';
import 'package:crm_employee/features/leads/domain/usecases/get_lead_detail_usecase.dart';
import 'package:crm_employee/features/leads/domain/usecases/launch_dialer_usecase.dart';
import 'package:crm_employee/features/leads/domain/usecases/launch_whatsapp_usecase.dart';
import 'package:crm_employee/features/leads/domain/usecases/log_call_usecase.dart';
import 'package:crm_employee/features/leads/domain/usecases/log_whatsapp_opened_usecase.dart';
import 'package:crm_employee/features/leads/domain/usecases/update_lead_status_usecase.dart';
import 'package:crm_employee/features/leads/presentation/bloc/lead_detail_bloc.dart';
import 'package:crm_employee/features/leads/presentation/bloc/lead_detail_event.dart';
import 'package:crm_employee/features/leads/presentation/bloc/lead_detail_state.dart';
import 'package:dartz/dartz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockGetLeadDetailUseCase extends Mock implements GetLeadDetailUseCase {}

class MockGetLeadActivitiesUseCase extends Mock
    implements GetLeadActivitiesUseCase {}

class MockUpdateLeadStatusUseCase extends Mock
    implements UpdateLeadStatusUseCase {}

class MockAddLeadNoteUseCase extends Mock implements AddLeadNoteUseCase {}

class MockLogCallUseCase extends Mock implements LogCallUseCase {}

class MockLogWhatsAppOpenedUseCase extends Mock
    implements LogWhatsAppOpenedUseCase {}

class MockLaunchDialerUseCase extends Mock implements LaunchDialerUseCase {}

class MockLaunchWhatsAppUseCase extends Mock
    implements LaunchWhatsAppUseCase {}

// Same reasoning as leads_bloc_test.dart — a real AuthBloc wired to
// mocked (never-called) use cases, since LeadDetailBloc only ever calls
// authBloc.add(...), never inspects its state.
class MockCheckStoredSessionUseCase extends Mock
    implements CheckStoredSessionUseCase {}

class MockCheckBiometricAvailabilityUseCase extends Mock
    implements CheckBiometricAvailabilityUseCase {}

class MockAuthenticateWithBiometricsUseCase extends Mock
    implements AuthenticateWithBiometricsUseCase {}

class MockLoginUseCase extends Mock implements LoginUseCase {}

class MockLogoutUseCase extends Mock implements LogoutUseCase {}

class MockGetCurrentUserUseCase extends Mock
    implements GetCurrentUserUseCase {}

Lead _lead({
  String id = 'l1',
  String status = 'new',
  int version = 1,
  String? phone,
  String? phoneE164,
}) => Lead(
  id: id,
  leadNumber: 1024,
  name: 'Rina Wijaya',
  phone: phone,
  phoneE164: phoneE164,
  status: status,
  source: 'manual',
  version: version,
  createdAt: DateTime(2026, 8, 30),
  updatedAt: DateTime(2026, 8, 30),
);

Activity _activity({String id = 'a1', String type = 'lead_created'}) =>
    Activity(
      id: id,
      leadId: 'l1',
      type: type,
      createdAt: DateTime(2026, 8, 30),
    );

void main() {
  late MockGetLeadDetailUseCase getLeadDetail;
  late MockGetLeadActivitiesUseCase getLeadActivities;
  late MockUpdateLeadStatusUseCase updateLeadStatus;
  late MockAddLeadNoteUseCase addLeadNote;
  late MockLogCallUseCase logCall;
  late MockLogWhatsAppOpenedUseCase logWhatsAppOpened;
  late MockLaunchDialerUseCase launchDialer;
  late MockLaunchWhatsAppUseCase launchWhatsApp;
  late AuthBloc authBloc;

  setUpAll(() {
    registerFallbackValue(
      const UpdateLeadStatusParams(leadId: 'l1', version: 1, status: 'contacted'),
    );
    registerFallbackValue(const AddLeadNoteParams(leadId: 'l1', body: 'x'));
    registerFallbackValue(const LogCallParams(leadId: 'l1', phone: '0812'));
  });

  setUp(() {
    getLeadDetail = MockGetLeadDetailUseCase();
    getLeadActivities = MockGetLeadActivitiesUseCase();
    updateLeadStatus = MockUpdateLeadStatusUseCase();
    addLeadNote = MockAddLeadNoteUseCase();
    logCall = MockLogCallUseCase();
    logWhatsAppOpened = MockLogWhatsAppOpenedUseCase();
    launchDialer = MockLaunchDialerUseCase();
    launchWhatsApp = MockLaunchWhatsAppUseCase();
    authBloc = AuthBloc(
      checkStoredSession: MockCheckStoredSessionUseCase(),
      checkBiometricAvailability: MockCheckBiometricAvailabilityUseCase(),
      authenticateWithBiometrics: MockAuthenticateWithBiometricsUseCase(),
      login: MockLoginUseCase(),
      logout: MockLogoutUseCase(),
      getCurrentUser: MockGetCurrentUserUseCase(),
    );

    // Default: activities load empty unless a test overrides it.
    when(
      () => getLeadActivities(any()),
    ).thenAnswer((_) async => const Right(ActivityListResult(activities: [], fromCache: false)));
  });

  tearDown(() => authBloc.close());

  LeadDetailBloc buildBloc() => LeadDetailBloc(
    getLeadDetail: getLeadDetail,
    getLeadActivities: getLeadActivities,
    updateLeadStatus: updateLeadStatus,
    addLeadNote: addLeadNote,
    logCall: logCall,
    logWhatsAppOpened: logWhatsAppOpened,
    launchDialer: launchDialer,
    launchWhatsApp: launchWhatsApp,
    authBloc: authBloc,
  );

  group('LeadDetailRequested', () {
    blocTest<LeadDetailBloc, LeadDetailState>(
      'loads lead + activities concurrently and reaches LeadDetailLoaded',
      setUp: () {
        when(() => getLeadDetail(any())).thenAnswer(
          (_) async => Right(LeadDetailResult(lead: _lead(), fromCache: false)),
        );
        when(() => getLeadActivities(any())).thenAnswer(
          (_) async => Right(
            ActivityListResult(activities: [_activity()], fromCache: false),
          ),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadDetailRequested('l1')),
      expect: () => [
        const LeadDetailLoading(),
        isA<LeadDetailLoaded>()
            .having((s) => s.lead.id, 'lead.id', 'l1')
            .having((s) => s.activities, 'activities', hasLength(1))
            .having((s) => s.fromCache, 'fromCache', isFalse),
      ],
    );

    blocTest<LeadDetailBloc, LeadDetailState>(
      'a non-session failure on either call emits LeadDetailError',
      setUp: () {
        when(() => getLeadDetail(any())).thenAnswer(
          (_) async => const Left(UnexpectedFailure('Lead tidak ditemukan.')),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadDetailRequested('l1')),
      expect: () => [
        const LeadDetailLoading(),
        const LeadDetailError('l1', 'Lead tidak ditemukan.'),
      ],
    );

    blocTest<LeadDetailBloc, LeadDetailState>(
      'SessionExpiredFailure dispatches AuthSessionInvalidated instead of emitting LeadDetailError',
      setUp: () {
        when(() => getLeadDetail(any())).thenAnswer(
          (_) async => const Left(SessionExpiredFailure('Sesi Anda berakhir.')),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadDetailRequested('l1')),
      expect: () => [const LeadDetailLoading()],
      verify: (_) async {
        await Future<void>.delayed(const Duration(milliseconds: 50));
        expect(authBloc.state, isA<AuthSessionExpired>());
      },
    );
  });

  group('LeadStatusChangeRequested', () {
    blocTest<LeadDetailBloc, LeadDetailState>(
      'a successful change refreshes activities and clears isUpdatingStatus',
      seed: () => LeadDetailLoaded(
        lead: _lead(),
        activities: const [],
        fromCache: false,
      ),
      setUp: () {
        when(() => updateLeadStatus(any())).thenAnswer(
          (_) async => Right(_lead(status: 'contacted', version: 2)),
        );
        when(() => getLeadActivities(any())).thenAnswer(
          (_) async => Right(
            ActivityListResult(
              activities: [_activity(type: 'status_changed')],
              fromCache: false,
            ),
          ),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadStatusChangeRequested('contacted')),
      expect: () => [
        isA<LeadDetailLoaded>().having(
          (s) => s.isUpdatingStatus,
          'isUpdatingStatus',
          isTrue,
        ),
        isA<LeadDetailLoaded>()
            .having((s) => s.lead.status, 'lead.status', 'contacted')
            .having((s) => s.lead.version, 'lead.version', 2)
            .having((s) => s.isUpdatingStatus, 'isUpdatingStatus', isFalse)
            .having((s) => s.activities, 'activities', hasLength(1)),
      ],
    );

    blocTest<LeadDetailBloc, LeadDetailState>(
      'a 409 version_conflict surfaces the dialog, never silently overwrites',
      seed: () => LeadDetailLoaded(
        lead: _lead(),
        activities: const [],
        fromCache: false,
      ),
      setUp: () {
        when(() => updateLeadStatus(any())).thenAnswer(
          (_) async => Left(
            VersionConflictFailure<Lead>(
              'Data sudah diubah oleh orang lain. Muat ulang dan coba lagi.',
              _lead(status: 'won', version: 3),
            ),
          ),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadStatusChangeRequested('contacted')),
      expect: () => [
        isA<LeadDetailLoaded>().having(
          (s) => s.isUpdatingStatus,
          'isUpdatingStatus',
          isTrue,
        ),
        isA<LeadDetailLoaded>()
            .having((s) => s.isUpdatingStatus, 'isUpdatingStatus', isFalse)
            .having((s) => s.conflict?.status, 'conflict.status', 'won')
            // The lead on screen is untouched by the rejected write.
            .having((s) => s.lead.status, 'lead.status', 'new'),
      ],
      verify: (_) {
        verifyNever(() => getLeadActivities(any()));
      },
    );

    blocTest<LeadDetailBloc, LeadDetailState>(
      'LeadStatusConflictAcknowledged reloads from the server, never retries the rejected write',
      seed: () => LeadDetailLoaded(
        lead: _lead(),
        activities: const [],
        fromCache: false,
        conflict: _lead(status: 'won', version: 3),
      ),
      setUp: () {
        when(() => getLeadDetail(any())).thenAnswer(
          (_) async =>
              Right(LeadDetailResult(lead: _lead(status: 'won', version: 3), fromCache: false)),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadStatusConflictAcknowledged()),
      expect: () => [
        const LeadDetailLoading(),
        isA<LeadDetailLoaded>()
            .having((s) => s.lead.status, 'lead.status', 'won')
            .having((s) => s.conflict, 'conflict', isNull),
      ],
      verify: (_) {
        verifyNever(() => updateLeadStatus(any()));
      },
    );
  });

  group('LeadNoteSubmitted', () {
    blocTest<LeadDetailBloc, LeadDetailState>(
      'an empty body sets noteError without calling the use case',
      seed: () => LeadDetailLoaded(
        lead: _lead(),
        activities: const [],
        fromCache: false,
      ),
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadNoteSubmitted('   ')),
      expect: () => [
        isA<LeadDetailLoaded>().having(
          (s) => s.noteError,
          'noteError',
          isNotNull,
        ),
      ],
      verify: (_) => verifyNever(() => addLeadNote(any())),
    );

    blocTest<LeadDetailBloc, LeadDetailState>(
      'a successful note submission clears the form and refreshes activities',
      seed: () => LeadDetailLoaded(
        lead: _lead(),
        activities: const [],
        fromCache: false,
      ),
      setUp: () {
        when(() => addLeadNote(any())).thenAnswer(
          (_) async => Right(_activity(type: 'note_added')),
        );
        when(() => getLeadActivities(any())).thenAnswer(
          (_) async => Right(
            ActivityListResult(
              activities: [_activity(type: 'note_added')],
              fromCache: false,
            ),
          ),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadNoteSubmitted('Follow up besok')),
      expect: () => [
        isA<LeadDetailLoaded>().having(
          (s) => s.isSubmittingNote,
          'isSubmittingNote',
          isTrue,
        ),
        isA<LeadDetailLoaded>()
            .having((s) => s.isSubmittingNote, 'isSubmittingNote', isFalse)
            .having((s) => s.activities, 'activities', hasLength(1)),
      ],
      verify: (_) {
        verify(
          () => addLeadNote(
            const AddLeadNoteParams(leadId: 'l1', body: 'Follow up besok'),
          ),
        ).called(1);
      },
    );
  });

  group('LeadCallRequested', () {
    blocTest<LeadDetailBloc, LeadDetailState>(
      'no phone on the lead is a defensive no-op — never calls launchDialer',
      seed: () => LeadDetailLoaded(
        lead: _lead(phone: null),
        activities: const [],
        fromCache: false,
      ),
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadCallRequested()),
      expect: () => <LeadDetailState>[],
      verify: (_) => verifyNever(() => launchDialer(any())),
    );

    blocTest<LeadDetailBloc, LeadDetailState>(
      'a canceled launch (OS never handed off) logs nothing — design brief §8.3',
      seed: () => LeadDetailLoaded(
        lead: _lead(phone: '0812'),
        activities: const [],
        fromCache: false,
      ),
      setUp: () {
        when(() => launchDialer('0812')).thenAnswer((_) async => false);
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadCallRequested()),
      expect: () => [
        isA<LeadDetailLoaded>().having(
          (s) => s.isLaunchingExternalAction,
          'isLaunchingExternalAction',
          isTrue,
        ),
        isA<LeadDetailLoaded>().having(
          (s) => s.isLaunchingExternalAction,
          'isLaunchingExternalAction',
          isFalse,
        ),
      ],
      verify: (_) => verifyNever(() => logCall(any())),
    );

    blocTest<LeadDetailBloc, LeadDetailState>(
      'a confirmed launch logs call_logged only AFTER the dialer actually opened',
      seed: () => LeadDetailLoaded(
        lead: _lead(phone: '0812'),
        activities: const [],
        fromCache: false,
      ),
      setUp: () {
        when(() => launchDialer('0812')).thenAnswer((_) async => true);
        when(() => logCall(any())).thenAnswer(
          (_) async => Right(_activity(type: 'call_logged')),
        );
        when(() => getLeadActivities(any())).thenAnswer(
          (_) async => Right(
            ActivityListResult(
              activities: [_activity(type: 'call_logged')],
              fromCache: false,
            ),
          ),
        );
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadCallRequested()),
      expect: () => [
        isA<LeadDetailLoaded>().having(
          (s) => s.isLaunchingExternalAction,
          'isLaunchingExternalAction',
          isTrue,
        ),
        isA<LeadDetailLoaded>().having(
          (s) => s.activities.first.type,
          'activities.first.type',
          'call_logged',
        ),
      ],
      verify: (_) {
        verify(
          () => logCall(const LogCallParams(leadId: 'l1', phone: '0812')),
        ).called(1);
      },
    );
  });

  group('LeadWhatsAppRequested', () {
    blocTest<LeadDetailBloc, LeadDetailState>(
      'no phoneE164 on the lead is a defensive no-op — never calls launchWhatsApp',
      seed: () => LeadDetailLoaded(
        lead: _lead(phoneE164: null),
        activities: const [],
        fromCache: false,
      ),
      build: buildBloc,
      act: (bloc) => bloc.add(const LeadWhatsAppRequested()),
      expect: () => <LeadDetailState>[],
      verify: (_) => verifyNever(() => launchWhatsApp(any())),
    );
  });
}
