import 'package:bloc_test/bloc_test.dart';
import 'package:crm_employee/core/error/failures.dart';
import 'package:crm_employee/core/usecases/usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/authenticate_with_biometrics_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/check_biometric_availability_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/check_stored_session_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/get_current_user_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/login_usecase.dart';
import 'package:crm_employee/features/auth/domain/usecases/logout_usecase.dart';
import 'package:crm_employee/features/auth/presentation/bloc/auth_bloc.dart';
import 'package:crm_employee/features/notifications/domain/entities/notification.dart';
import 'package:crm_employee/features/notifications/domain/usecases/get_notifications_usecase.dart';
import 'package:crm_employee/features/notifications/domain/usecases/mark_notification_read_usecase.dart';
import 'package:crm_employee/features/notifications/presentation/bloc/notifications_bloc.dart';
import 'package:crm_employee/features/notifications/presentation/bloc/notifications_event.dart';
import 'package:crm_employee/features/notifications/presentation/bloc/notifications_state.dart';
import 'package:dartz/dartz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockGetNotificationsUseCase extends Mock
    implements GetNotificationsUseCase {}

class MockMarkNotificationReadUseCase extends Mock
    implements MarkNotificationReadUseCase {}

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

NotificationItem _item({String id = 'n1', DateTime? readAt}) =>
    NotificationItem(
      id: id,
      type: 'lead_assigned',
      leadId: 'l1',
      title: 'Lead baru',
      body: 'Rina Wijaya ditugaskan ke Anda',
      readAt: readAt,
      createdAt: DateTime(2026, 8, 30),
    );

void main() {
  late MockGetNotificationsUseCase getNotifications;
  late MockMarkNotificationReadUseCase markNotificationRead;
  late AuthBloc authBloc;

  setUpAll(() {
    registerFallbackValue(const NoParams());
  });

  setUp(() {
    getNotifications = MockGetNotificationsUseCase();
    markNotificationRead = MockMarkNotificationReadUseCase();
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

  NotificationsBloc buildBloc() => NotificationsBloc(
    getNotifications: getNotifications,
    markNotificationRead: markNotificationRead,
    authBloc: authBloc,
  );

  blocTest<NotificationsBloc, NotificationsState>(
    'NotificationsRequested loads and reaches NotificationsLoaded',
    setUp: () {
      when(() => getNotifications(any())).thenAnswer(
        (_) async => Right(NotificationListResult(notifications: [_item()])),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const NotificationsRequested()),
    expect: () => [
      const NotificationsLoading(),
      isA<NotificationsLoaded>().having(
        (s) => s.notifications,
        'notifications',
        hasLength(1),
      ),
    ],
  );

  blocTest<NotificationsBloc, NotificationsState>(
    'a non-session failure emits NotificationsError',
    setUp: () {
      when(() => getNotifications(any())).thenAnswer(
        (_) async => const Left(UnexpectedFailure('Tidak dapat terhubung ke server. Periksa koneksi Anda.')),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const NotificationsRequested()),
    expect: () => [
      const NotificationsLoading(),
      const NotificationsError('Tidak dapat terhubung ke server. Periksa koneksi Anda.'),
    ],
  );

  blocTest<NotificationsBloc, NotificationsState>(
    'SessionExpiredFailure dispatches AuthSessionInvalidated instead of emitting NotificationsError',
    setUp: () {
      when(() => getNotifications(any())).thenAnswer(
        (_) async => const Left(SessionExpiredFailure('Sesi Anda berakhir.')),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const NotificationsRequested()),
    expect: () => [const NotificationsLoading()],
  );

  group('NotificationMarkReadRequested', () {
    blocTest<NotificationsBloc, NotificationsState>(
      'marks the matching row read optimistically, leaves others untouched',
      seed: () => NotificationsLoaded([_item(id: 'n1'), _item(id: 'n2')]),
      setUp: () {
        when(
          () => markNotificationRead('n1'),
        ).thenAnswer((_) async => const Right(null));
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const NotificationMarkReadRequested('n1')),
      expect: () => [
        isA<NotificationsLoaded>()
            .having(
              (s) => s.notifications.firstWhere((n) => n.id == 'n1').isUnread,
              'n1.isUnread',
              isFalse,
            )
            .having(
              (s) => s.notifications.firstWhere((n) => n.id == 'n2').isUnread,
              'n2.isUnread',
              isTrue,
            ),
      ],
    );

    blocTest<NotificationsBloc, NotificationsState>(
      'a failed markRead call leaves the optimistic update standing — never reverted, never an error banner',
      seed: () => NotificationsLoaded([_item(id: 'n1')]),
      setUp: () {
        when(
          () => markNotificationRead('n1'),
        ).thenAnswer((_) async => const Left(UnexpectedFailure('down')));
      },
      build: buildBloc,
      act: (bloc) => bloc.add(const NotificationMarkReadRequested('n1')),
      expect: () => [
        isA<NotificationsLoaded>().having(
          (s) => s.notifications.first.isUnread,
          'isUnread',
          isFalse,
        ),
      ],
    );
  });
}
