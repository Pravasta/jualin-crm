/// Mirrors `crm_dashboard/src/lib/api-types.ts`'s `ApiErrorBody`/`ApiError`
/// — the error envelope `{code, message, details}` (Aturan #33), read the
/// same way on both clients.
class ErrorDetail {
  final String field;
  final String code;

  const ErrorDetail({required this.field, required this.code});

  factory ErrorDetail.fromJson(Map<String, dynamic> json) {
    return ErrorDetail(
      field: json['field'] as String? ?? '',
      code: json['code'] as String? ?? '',
    );
  }
}

class ApiError implements Exception {
  final int status;
  final String code;
  final String message;
  final List<ErrorDetail>? details;

  /// Only set for `409 version_conflict` (Aturan #35) — the row's
  /// current state, raw JSON. Left undecoded here (this file has no
  /// business knowing what a "lead" is); `LeadRepositoryImpl` is what
  /// parses it into a domain `Lead` when mapping this into a
  /// `VersionConflictFailure`.
  final Map<String, dynamic>? current;

  const ApiError({
    required this.status,
    required this.code,
    required this.message,
    this.details,
    this.current,
  });

  factory ApiError.fromBody(int status, Map<String, dynamic> body) {
    final detailsJson = body['details'] as List<dynamic>?;
    return ApiError(
      status: status,
      code: body['code'] as String? ?? 'internal_error',
      message: body['message'] as String? ?? 'Terjadi kesalahan internal.',
      details: detailsJson
          ?.map((d) => ErrorDetail.fromJson(d as Map<String, dynamic>))
          .toList(),
      current: body['current'] as Map<String, dynamic>?,
    );
  }

  @override
  String toString() => message;
}

/// Thrown by [ApiClient] specifically when a session ends and cannot be
/// refreshed (expired/revoked refresh token, or refresh itself failing) —
/// distinct from [ApiError] so callers can catch this one case (redirect
/// to login) without pattern-matching on `code == 'authentication_required'`
/// at every call site.
class SessionExpiredException implements Exception {
  const SessionExpiredException();

  @override
  String toString() => 'Sesi Anda berakhir. Silakan masuk kembali.';
}
