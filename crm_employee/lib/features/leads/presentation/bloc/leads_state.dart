import 'package:equatable/equatable.dart';

import '../../domain/entities/lead.dart';

/// Every state carries the currently-active filter/query — so the chip
/// row and search box never flicker back to "no filter" while a reload
/// for a NEW filter is in flight; they show what the user picked, not
/// what's currently loaded.
sealed class LeadsState extends Equatable {
  final String? statusFilter;
  final String query;

  const LeadsState({this.statusFilter, this.query = ''});

  @override
  List<Object?> get props => [statusFilter, query];
}

class LeadsInitial extends LeadsState {
  const LeadsInitial();
}

class LeadsLoading extends LeadsState {
  const LeadsLoading({super.statusFilter, super.query});
}

class LeadsLoaded extends LeadsState {
  final List<Lead> leads;
  final int total;

  /// TD §7 — true when this came from the offline cache (network was
  /// unreachable), in which case [fetchedAt] is when that cached
  /// response was originally saved, not now.
  final bool fromCache;
  final DateTime? fetchedAt;

  const LeadsLoaded({
    required this.leads,
    required this.total,
    required this.fromCache,
    this.fetchedAt,
    super.statusFilter,
    super.query,
  });

  @override
  List<Object?> get props => [
    ...super.props,
    leads,
    total,
    fromCache,
    fetchedAt,
  ];
}

class LeadsError extends LeadsState {
  final String message;

  const LeadsError(this.message, {super.statusFilter, super.query});

  @override
  List<Object?> get props => [...super.props, message];
}
