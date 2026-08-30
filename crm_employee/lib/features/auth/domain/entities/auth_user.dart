import 'package:equatable/equatable.dart';

/// The logged-in user, as the domain/presentation layers see it — no
/// knowledge of JSON, HTTP, or the wire field names `crm_be` uses.
/// `AuthUserModel` (data layer) is what actually parses `/v1/me`'s
/// response into this.
class AuthUser extends Equatable {
  final String userId;
  final String email;
  final String fullName;
  final String organizationId;
  final String organizationName;
  final String membershipId;
  final String role;

  const AuthUser({
    required this.userId,
    required this.email,
    required this.fullName,
    required this.organizationId,
    required this.organizationName,
    required this.membershipId,
    required this.role,
  });

  @override
  List<Object?> get props => [
    userId,
    email,
    fullName,
    organizationId,
    organizationName,
    membershipId,
    role,
  ];
}
