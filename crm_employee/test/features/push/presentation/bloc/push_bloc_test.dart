// Focuses on the logic most likely to break silently: whether
// pendingLeadId ends up set from EITHER onMessageOpenedApp or a cold-start
// getInitialMessage, survives until consumed, and never gets set from a
// message with no lead_id.
import 'dart:async';

import 'package:bloc_test/bloc_test.dart';
import 'package:crm_employee/core/error/failures.dart';
import 'package:crm_employee/core/usecases/usecase.dart';
import 'package:crm_employee/features/push/domain/entities/push_message.dart';
import 'package:crm_employee/features/push/domain/usecases/observe_push_messages_usecase.dart';
import 'package:crm_employee/features/push/domain/usecases/register_device_token_usecase.dart';
import 'package:crm_employee/features/push/domain/usecases/request_notification_permission_usecase.dart';
import 'package:crm_employee/features/push/presentation/bloc/push_bloc.dart';
import 'package:crm_employee/features/push/presentation/bloc/push_event.dart';
import 'package:crm_employee/features/push/presentation/bloc/push_state.dart';
import 'package:dartz/dartz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockRequestNotificationPermissionUseCase extends Mock
    implements RequestNotificationPermissionUseCase {}

class MockRegisterDeviceTokenUseCase extends Mock
    implements RegisterDeviceTokenUseCase {}

class MockObservePushMessagesUseCase extends Mock
    implements ObservePushMessagesUseCase {}

void main() {
  late MockRequestNotificationPermissionUseCase requestPermission;
  late MockRegisterDeviceTokenUseCase registerDeviceToken;
  late MockObservePushMessagesUseCase observeMessages;
  late StreamController<PushMessage> foregroundController;
  late StreamController<PushMessage> tappedController;
  late StreamController<String> tokenRefreshController;

  setUpAll(() {
    registerFallbackValue(const NoParams());
  });

  setUp(() {
    requestPermission = MockRequestNotificationPermissionUseCase();
    registerDeviceToken = MockRegisterDeviceTokenUseCase();
    observeMessages = MockObservePushMessagesUseCase();
    foregroundController = StreamController<PushMessage>.broadcast();
    tappedController = StreamController<PushMessage>.broadcast();
    tokenRefreshController = StreamController<String>.broadcast();

    when(() => requestPermission()).thenAnswer((_) async => true);
    when(() => observeMessages.getInitialMessage()).thenAnswer((_) async => null);
    when(
      () => observeMessages.onForegroundMessage,
    ).thenAnswer((_) => foregroundController.stream);
    when(
      () => observeMessages.onMessageOpenedApp,
    ).thenAnswer((_) => tappedController.stream);
    when(
      () => observeMessages.onTokenRefresh,
    ).thenAnswer((_) => tokenRefreshController.stream);
    when(
      () => registerDeviceToken(any()),
    ).thenAnswer((_) async => const Right(null));
  });

  tearDown(() async {
    await foregroundController.close();
    await tappedController.close();
    await tokenRefreshController.close();
  });

  PushBloc buildBloc() => PushBloc(
    requestPermission: requestPermission,
    registerDeviceToken: registerDeviceToken,
    observeMessages: observeMessages,
  );

  blocTest<PushBloc, PushState>(
    'PushInitialized with no cold-start message leaves pendingLeadId null',
    build: buildBloc,
    act: (bloc) => bloc.add(const PushInitialized()),
    expect: () => <PushState>[],
    verify: (_) => verify(() => requestPermission()).called(1),
  );

  blocTest<PushBloc, PushState>(
    'a cold-start getInitialMessage with a lead_id sets pendingLeadId',
    setUp: () {
      when(() => observeMessages.getInitialMessage()).thenAnswer(
        (_) async => const PushMessage(leadId: 'l1', title: 't', body: 'b'),
      );
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const PushInitialized()),
    expect: () => [
      const PushState(pendingLeadId: 'l1'),
    ],
  );

  blocTest<PushBloc, PushState>(
    'onMessageOpenedApp (background, tapped) sets pendingLeadId',
    build: buildBloc,
    act: (bloc) async {
      bloc.add(const PushInitialized());
      await Future<void>.delayed(Duration.zero);
      tappedController.add(const PushMessage(leadId: 'l2'));
    },
    expect: () => [const PushState(pendingLeadId: 'l2')],
  );

  blocTest<PushBloc, PushState>(
    'a tapped message with no lead_id is a no-op — never sets pendingLeadId',
    build: buildBloc,
    act: (bloc) async {
      bloc.add(const PushInitialized());
      await Future<void>.delayed(Duration.zero);
      tappedController.add(const PushMessage());
    },
    expect: () => <PushState>[],
  );

  blocTest<PushBloc, PushState>(
    'a foreground message sets foregroundMessage, never pendingLeadId — design brief §10: banner, not navigation',
    build: buildBloc,
    act: (bloc) async {
      bloc.add(const PushInitialized());
      await Future<void>.delayed(Duration.zero);
      foregroundController.add(const PushMessage(leadId: 'l3', title: 't', body: 'b'));
    },
    expect: () => [
      isA<PushState>()
          .having((s) => s.pendingLeadId, 'pendingLeadId', isNull)
          .having((s) => s.foregroundMessage?.leadId, 'foregroundMessage.leadId', 'l3'),
    ],
  );

  blocTest<PushBloc, PushState>(
    'PushDeeplinkConsumed clears pendingLeadId but keeps foregroundMessage untouched',
    seed: () => const PushState(
      pendingLeadId: 'l1',
      foregroundMessage: PushMessage(leadId: 'l2'),
    ),
    build: buildBloc,
    act: (bloc) => bloc.add(const PushDeeplinkConsumed()),
    expect: () => [
      const PushState(foregroundMessage: PushMessage(leadId: 'l2')),
    ],
  );

  blocTest<PushBloc, PushState>(
    'PushForegroundBannerDismissed clears foregroundMessage but keeps pendingLeadId untouched',
    seed: () => const PushState(
      pendingLeadId: 'l1',
      foregroundMessage: PushMessage(leadId: 'l2'),
    ),
    build: buildBloc,
    act: (bloc) => bloc.add(const PushForegroundBannerDismissed()),
    expect: () => [const PushState(pendingLeadId: 'l1')],
  );

  blocTest<PushBloc, PushState>(
    'a token refresh re-triggers registration',
    build: buildBloc,
    act: (bloc) async {
      bloc.add(const PushInitialized());
      await Future<void>.delayed(Duration.zero);
      tokenRefreshController.add('new-token');
      await Future<void>.delayed(Duration.zero);
    },
    expect: () => <PushState>[],
    verify: (_) => verify(() => registerDeviceToken(any())).called(1),
  );

  blocTest<PushBloc, PushState>(
    'PushRegistrationRequested calls the use case — a failure never crashes the bloc or emits an error state',
    setUp: () {
      when(
        () => registerDeviceToken(any()),
      ).thenAnswer((_) async => const Left(UnexpectedFailure('no token')));
    },
    build: buildBloc,
    act: (bloc) => bloc.add(const PushRegistrationRequested()),
    expect: () => <PushState>[],
  );
}
