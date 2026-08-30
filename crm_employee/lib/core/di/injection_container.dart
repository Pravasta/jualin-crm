import 'package:get_it/get_it.dart';

import '../../features/auth/data/datasources/auth_remote_data_source.dart';
import '../../features/auth/data/datasources/biometric_local_data_source.dart';
import '../../features/auth/data/repositories/auth_repository_impl.dart';
import '../../features/auth/data/repositories/biometric_repository_impl.dart';
import '../../features/auth/domain/repositories/auth_repository.dart';
import '../../features/auth/domain/repositories/biometric_repository.dart';
import '../../features/auth/domain/usecases/authenticate_with_biometrics_usecase.dart';
import '../../features/auth/domain/usecases/check_biometric_availability_usecase.dart';
import '../../features/auth/domain/usecases/check_stored_session_usecase.dart';
import '../../features/auth/domain/usecases/get_current_user_usecase.dart';
import '../../features/auth/domain/usecases/login_usecase.dart';
import '../../features/auth/domain/usecases/logout_usecase.dart';
import '../../features/auth/presentation/bloc/auth_bloc.dart';
import '../../features/leads/data/datasources/activity_remote_data_source.dart';
import '../../features/leads/data/datasources/external_app_data_source.dart';
import '../../features/leads/data/datasources/lead_remote_data_source.dart';
import '../../features/leads/data/repositories/activity_repository_impl.dart';
import '../../features/leads/data/repositories/external_action_repository_impl.dart';
import '../../features/leads/data/repositories/lead_repository_impl.dart';
import '../../features/leads/domain/repositories/activity_repository.dart';
import '../../features/leads/domain/repositories/external_action_repository.dart';
import '../../features/leads/domain/repositories/lead_repository.dart';
import '../../features/leads/domain/usecases/add_lead_note_usecase.dart';
import '../../features/leads/domain/usecases/get_lead_activities_usecase.dart';
import '../../features/leads/domain/usecases/get_lead_detail_usecase.dart';
import '../../features/leads/domain/usecases/get_my_leads_usecase.dart';
import '../../features/leads/domain/usecases/launch_dialer_usecase.dart';
import '../../features/leads/domain/usecases/launch_whatsapp_usecase.dart';
import '../../features/leads/domain/usecases/log_call_usecase.dart';
import '../../features/leads/domain/usecases/log_whatsapp_opened_usecase.dart';
import '../../features/leads/domain/usecases/update_lead_status_usecase.dart';
import '../../features/leads/presentation/bloc/lead_detail_bloc.dart';
import '../../features/leads/presentation/bloc/leads_bloc.dart';
import '../../features/notifications/data/datasources/notification_remote_data_source.dart';
import '../../features/notifications/data/repositories/notification_repository_impl.dart';
import '../../features/notifications/domain/repositories/notification_repository.dart';
import '../../features/notifications/domain/usecases/get_notifications_usecase.dart';
import '../../features/notifications/domain/usecases/mark_notification_read_usecase.dart';
import '../../features/notifications/presentation/bloc/notifications_bloc.dart';
import '../../features/push/data/datasources/firebase_messaging_data_source.dart';
import '../../features/push/data/repositories/push_repository_impl.dart';
import '../../features/push/domain/repositories/push_repository.dart';
import '../../features/push/domain/usecases/observe_push_messages_usecase.dart';
import '../../features/push/domain/usecases/register_device_token_usecase.dart';
import '../../features/push/domain/usecases/request_notification_permission_usecase.dart';
import '../../features/push/presentation/bloc/push_bloc.dart';
import '../../features/tasks/data/datasources/task_remote_data_source.dart';
import '../../features/tasks/data/repositories/task_repository_impl.dart';
import '../../features/tasks/domain/repositories/task_repository.dart';
import '../../features/tasks/domain/usecases/complete_task_usecase.dart';
import '../../features/tasks/domain/usecases/get_my_tasks_usecase.dart';
import '../../features/tasks/presentation/bloc/tasks_bloc.dart';
import '../api_client.dart';
import '../cache/response_cache.dart';
import '../cache/sqflite_response_cache.dart';
import '../push/device_token_remote_data_source.dart';
import '../push/push_token_store.dart';
import '../secure_store.dart';

/// Composition root — every service is constructed exactly once here and
/// resolved through `get_it`, the same role `cmd/api/main.go` plays for
/// `crm_be`'s dependencies at boot (ADR-011's composition root, applied
/// to a Flutter app instead of a Go binary). `init()` runs once, from
/// `main()`, before `runApp`.
final GetIt sl = GetIt.instance;

Future<void> initDependencyInjection() async {
  // --- core ---
  sl.registerLazySingleton<TokenStorage>(() => const SecureTokenStorage());
  sl.registerLazySingleton<ApiClient>(() => ApiClient(tokens: sl()));
  // A single cache instance/connection for the whole app (TD §7) — every
  // feature that caches GET responses shares the same SQLite database
  // and, correspondingly, the same "clear everything on logout".
  sl.registerLazySingleton<ResponseCache>(() => SqfliteResponseCache());
  // Shared by `auth` (unregister on logout) and `push` (register after
  // login) without either importing the other's `features/` tree — see
  // device_token_remote_data_source.dart's doc comment.
  sl.registerLazySingleton<DeviceTokenRemoteDataSource>(
    () => DeviceTokenRemoteDataSourceImpl(sl()),
  );
  sl.registerLazySingleton<PushTokenStore>(() => const SecurePushTokenStore());

  // --- feature: auth — data sources ---
  sl.registerLazySingleton<AuthRemoteDataSource>(
    () => AuthRemoteDataSourceImpl(sl()),
  );
  sl.registerLazySingleton<BiometricLocalDataSource>(
    () => BiometricLocalDataSourceImpl(),
  );

  // --- feature: auth — repositories ---
  sl.registerLazySingleton<AuthRepository>(
    () => AuthRepositoryImpl(
      remoteDataSource: sl(),
      tokenStorage: sl(),
      responseCache: sl(),
      deviceTokenRemoteDataSource: sl(),
      pushTokenStore: sl(),
    ),
  );
  sl.registerLazySingleton<BiometricRepository>(
    () => BiometricRepositoryImpl(sl()),
  );

  // --- feature: auth — use cases ---
  sl.registerLazySingleton(() => LoginUseCase(sl()));
  sl.registerLazySingleton(() => LogoutUseCase(sl()));
  sl.registerLazySingleton(() => GetCurrentUserUseCase(sl()));
  sl.registerLazySingleton(() => CheckStoredSessionUseCase(sl()));
  sl.registerLazySingleton(() => CheckBiometricAvailabilityUseCase(sl()));
  sl.registerLazySingleton(() => AuthenticateWithBiometricsUseCase(sl()));

  // --- feature: auth — bloc ---
  // One instance for the whole app lifetime (registered the same way as
  // the services above), not one per widget subtree — session state is
  // global, not scoped to wherever AuthGatePage happens to be mounted.
  sl.registerLazySingleton(
    () => AuthBloc(
      checkStoredSession: sl(),
      checkBiometricAvailability: sl(),
      authenticateWithBiometrics: sl(),
      login: sl(),
      logout: sl(),
      getCurrentUser: sl(),
    ),
  );

  // --- feature: leads — data sources, repository, use case ---
  sl.registerLazySingleton<LeadRemoteDataSource>(
    () => LeadRemoteDataSourceImpl(sl()),
  );
  sl.registerLazySingleton<LeadRepository>(
    () => LeadRepositoryImpl(remoteDataSource: sl(), responseCache: sl()),
  );
  sl.registerLazySingleton(() => GetMyLeadsUseCase(sl()));

  // --- feature: leads — bloc ---
  // registerFactory, unlike AuthBloc — this is per-screen state (the tab
  // being alive), not app-wide session state. AppShell's IndexedStack
  // still only ever asks for one instance in practice (its tab list is
  // built once via `late final`), but factory is the architecturally
  // honest choice for what this bloc actually represents.
  sl.registerFactory(() => LeadsBloc(getMyLeads: sl(), authBloc: sl()));

  // --- feature: leads (detail, #72) — data sources ---
  sl.registerLazySingleton<ActivityRemoteDataSource>(
    () => ActivityRemoteDataSourceImpl(sl()),
  );
  sl.registerLazySingleton<ExternalAppDataSource>(
    () => ExternalAppDataSourceImpl(),
  );

  // --- feature: leads (detail, #72) — repositories ---
  sl.registerLazySingleton<ActivityRepository>(
    () => ActivityRepositoryImpl(remoteDataSource: sl(), responseCache: sl()),
  );
  sl.registerLazySingleton<ExternalActionRepository>(
    () => ExternalActionRepositoryImpl(sl()),
  );

  // --- feature: leads (detail, #72) — use cases ---
  sl.registerLazySingleton(() => GetLeadDetailUseCase(sl()));
  sl.registerLazySingleton(() => GetLeadActivitiesUseCase(sl()));
  sl.registerLazySingleton(() => UpdateLeadStatusUseCase(sl()));
  sl.registerLazySingleton(() => AddLeadNoteUseCase(sl()));
  sl.registerLazySingleton(() => LogCallUseCase(sl()));
  sl.registerLazySingleton(() => LogWhatsAppOpenedUseCase(sl()));
  sl.registerLazySingleton(() => LaunchDialerUseCase(sl()));
  sl.registerLazySingleton(() => LaunchWhatsAppUseCase(sl()));

  // --- feature: leads (detail, #72) — bloc ---
  // registerFactory, same reasoning as LeadsBloc — one instance per time
  // Detail Lead is open, not app-wide state.
  sl.registerFactory(
    () => LeadDetailBloc(
      getLeadDetail: sl(),
      getLeadActivities: sl(),
      updateLeadStatus: sl(),
      addLeadNote: sl(),
      logCall: sl(),
      logWhatsAppOpened: sl(),
      launchDialer: sl(),
      launchWhatsApp: sl(),
      authBloc: sl(),
    ),
  );

  // --- feature: push (#73) ---
  sl.registerLazySingleton<FirebaseMessagingDataSource>(
    () => FirebaseMessagingDataSourceImpl(),
  );
  sl.registerLazySingleton<PushRepository>(
    () => PushRepositoryImpl(
      messagingDataSource: sl(),
      deviceTokenRemoteDataSource: sl(),
      pushTokenStore: sl(),
    ),
  );
  sl.registerLazySingleton(() => RequestNotificationPermissionUseCase(sl()));
  sl.registerLazySingleton(() => RegisterDeviceTokenUseCase(sl()));
  sl.registerLazySingleton(() => ObservePushMessagesUseCase(sl()));
  // App-wide, one instance for the whole app lifetime — same reasoning
  // as AuthBloc: a push arriving/being tapped is a global concern, not
  // scoped to whatever screen happens to be showing.
  sl.registerLazySingleton(
    () => PushBloc(
      requestPermission: sl(),
      registerDeviceToken: sl(),
      observeMessages: sl(),
    ),
  );

  // --- feature: tasks (#73) ---
  sl.registerLazySingleton<TaskRemoteDataSource>(
    () => TaskRemoteDataSourceImpl(sl()),
  );
  sl.registerLazySingleton<TaskRepository>(
    () => TaskRepositoryImpl(remoteDataSource: sl(), responseCache: sl()),
  );
  sl.registerLazySingleton(() => GetMyTasksUseCase(sl()));
  sl.registerLazySingleton(() => CompleteTaskUseCase(sl()));
  // registerFactory — same reasoning as LeadsBloc: per-tab state, not
  // app-wide.
  sl.registerFactory(
    () => TasksBloc(getMyTasks: sl(), completeTask: sl(), authBloc: sl()),
  );

  // --- feature: notifications (#73) ---
  sl.registerLazySingleton<NotificationRemoteDataSource>(
    () => NotificationRemoteDataSourceImpl(sl()),
  );
  sl.registerLazySingleton<NotificationRepository>(
    () => NotificationRepositoryImpl(remoteDataSource: sl()),
  );
  sl.registerLazySingleton(() => GetNotificationsUseCase(sl()));
  sl.registerLazySingleton(() => MarkNotificationReadUseCase(sl()));
  sl.registerFactory(
    () => NotificationsBloc(
      getNotifications: sl(),
      markNotificationRead: sl(),
      authBloc: sl(),
    ),
  );
}
