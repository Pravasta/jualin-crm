import 'package:equatable/equatable.dart';

sealed class LeadsEvent extends Equatable {
  const LeadsEvent();

  @override
  List<Object?> get props => [];
}

/// Dispatched once when the tab is first shown.
class LeadsRequested extends LeadsEvent {
  const LeadsRequested();
}

/// The status chip row (design brief §6) — single-select, `null` for
/// "Semua".
class LeadsStatusFilterChanged extends LeadsEvent {
  final String? status;

  const LeadsStatusFilterChanged(this.status);

  @override
  List<Object?> get props => [status];
}

/// Dispatched by the search box AFTER its own debounce settles (the
/// widget owns the `Timer`, same pattern `crm_dashboard`'s #32 used for
/// its 300ms debounce) — the bloc itself has no notion of "typing".
class LeadsSearchChanged extends LeadsEvent {
  final String query;

  const LeadsSearchChanged(this.query);

  @override
  List<Object?> get props => [query];
}

/// Pull-to-refresh — re-runs the current filter/query, always trying the
/// network first (never serves straight from cache on an explicit
/// refresh gesture).
class LeadsRefreshRequested extends LeadsEvent {
  const LeadsRefreshRequested();
}
