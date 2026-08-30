import 'package:dartz/dartz.dart';

import '../api_error.dart';
import '../error/failures.dart';

/// Runs an `ApiClient`-backed call and translates whatever it throws into
/// `Either<Failure, T>` — the exact three-way catch
/// `AuthRepositoryImpl.login`/`getCurrentUser` each wrote out by hand in
/// #69. Extracted here once `LeadRepositoryImpl` needed the identical
/// shape (#71) — Aturan #28's "abstraksi hanya setelah implementasi
/// kedua yang nyata": this is that second implementation, not a
/// speculative one.
///
/// [onApiError] lets a caller override the generic mapping for a
/// specific `ApiError.code` it cares about (e.g. `login`'s
/// `invalid_credentials` → `InvalidCredentialsFailure`) — every other
/// code still falls through to `UnexpectedFailure`.
Future<Either<Failure, T>> runApiCall<T>(
  Future<T> Function() call, {
  Failure Function(ApiError)? onApiError,
}) async {
  try {
    return Right(await call());
  } on SessionExpiredException catch (e) {
    return Left(SessionExpiredFailure(e.toString()));
  } on ApiError catch (e) {
    return Left(onApiError?.call(e) ?? UnexpectedFailure(e.message));
  } catch (_) {
    return const Left(
      UnexpectedFailure('Tidak dapat terhubung ke server. Periksa koneksi Anda.'),
    );
  }
}
