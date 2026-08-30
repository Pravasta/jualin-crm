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
import '../../features/leads/data/datasources/lead_remote_data_source.dart';
import '../../features/leads/data/repositories/lead_repository_impl.dart';
import '../../features/leads/domain/repositories/lead_repository.dart';
import '../../features/leads/domain/usecases/get_my_leads_usecase.dart';
import '../../features/leads/presentation/bloc/leads_bloc.dart';
import '../api_client.dart';
import '../cache/response_cache.dart';
import '../cache/sqflite_response_cache.dart';
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
}
