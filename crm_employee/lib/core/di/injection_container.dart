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
import '../api_client.dart';
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

  // --- feature: auth — data sources ---
  sl.registerLazySingleton<AuthRemoteDataSource>(
    () => AuthRemoteDataSourceImpl(sl()),
  );
  sl.registerLazySingleton<BiometricLocalDataSource>(
    () => BiometricLocalDataSourceImpl(),
  );

  // --- feature: auth — repositories ---
  sl.registerLazySingleton<AuthRepository>(
    () => AuthRepositoryImpl(remoteDataSource: sl(), tokenStorage: sl()),
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
}
