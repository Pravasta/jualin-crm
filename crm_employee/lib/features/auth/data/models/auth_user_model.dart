import '../../domain/entities/auth_user.dart';

/// Shape read directly from `crm_be/internal/auth/{handler_http,entity}.go`
/// — `GET /v1/me`'s response. `fromJson` is the only place in this
/// feature that knows the wire's `snake_case` field names.
class AuthUserModel extends AuthUser {
  const AuthUserModel({
    required super.userId,
    required super.email,
    required super.fullName,
    required super.organizationId,
    required super.organizationName,
    required super.membershipId,
    required super.role,
  });

  factory AuthUserModel.fromJson(Map<String, dynamic> json) {
    return AuthUserModel(
      userId: json['user_id'] as String,
      email: json['email'] as String,
      fullName: json['full_name'] as String,
      organizationId: json['organization_id'] as String,
      organizationName: json['organization_name'] as String,
      membershipId: json['membership_id'] as String,
      role: json['role'] as String,
    );
  }
}
