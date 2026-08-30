import 'package:equatable/equatable.dart';

/// Shape read from `crm_be/internal/lead/{handler_http,entity}.go`'s
/// `leadJSON` — only the fields My Leads (#71) and Detail Lead (#72)
/// actually use. No knowledge of JSON here; `LeadModel` (data layer) is
/// what parses the wire format into this.
class Lead extends Equatable {
  final String id;
  final int leadNumber;
  final String name;
  final String? email;
  final String? phone;
  final String? company;
  final String? notes;
  final String status;
  final String? lostReason;
  final String source;
  final String? assignedToMembershipId;
  final int version;
  final DateTime createdAt;
  final DateTime updatedAt;

  const Lead({
    required this.id,
    required this.leadNumber,
    required this.name,
    this.email,
    this.phone,
    this.company,
    this.notes,
    required this.status,
    this.lostReason,
    required this.source,
    this.assignedToMembershipId,
    required this.version,
    required this.createdAt,
    required this.updatedAt,
  });

  @override
  List<Object?> get props => [
    id,
    leadNumber,
    name,
    email,
    phone,
    company,
    notes,
    status,
    lostReason,
    source,
    assignedToMembershipId,
    version,
    createdAt,
    updatedAt,
  ];
}

/// One page of `GET /v1/leads` (Aturan #33's `{data, meta}` envelope) —
/// plus whether it came from the offline cache and, if so, when it was
/// fetched (TD §7's "Data terakhir diperbarui" banner).
class LeadListResult extends Equatable {
  final List<Lead> leads;
  final int total;
  final bool fromCache;
  final DateTime? fetchedAt;

  const LeadListResult({
    required this.leads,
    required this.total,
    required this.fromCache,
    this.fetchedAt,
  });

  @override
  List<Object?> get props => [leads, total, fromCache, fetchedAt];
}
