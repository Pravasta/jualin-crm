import 'package:equatable/equatable.dart';

sealed class LeadDetailEvent extends Equatable {
  const LeadDetailEvent();

  @override
  List<Object?> get props => [];
}

/// Dispatched once when the screen opens, with the id `LeadListItem.onTap`
/// (Lead Saya) navigated in with.
class LeadDetailRequested extends LeadDetailEvent {
  final String leadId;

  const LeadDetailRequested(this.leadId);

  @override
  List<Object?> get props => [leadId];
}

/// Pull-to-refresh — same contract as `LeadsRefreshRequested`: always
/// tries the network first, never serves straight from cache on an
/// explicit gesture. Reuses whatever lead id is already loaded, so the
/// widget doesn't need to remember it separately.
class LeadDetailRefreshRequested extends LeadDetailEvent {
  const LeadDetailRefreshRequested();
}

/// The status picker's chosen option (`lead_status.dart`'s
/// `statusTransitionOptions`) — `lostReason` is only set when `status ==
/// 'lost'`, after the design brief §9.2 follow-up step.
class LeadStatusChangeRequested extends LeadDetailEvent {
  final String status;
  final String? lostReason;

  const LeadStatusChangeRequested(this.status, {this.lostReason});

  @override
  List<Object?> get props => [status, lostReason];
}

/// The conflict dialog's (design brief §8.2) only exit — "muat ulang":
/// dismisses the dialog and reloads from the server. Never "try the same
/// write again" — the whole point is the lead this device knew about is
/// stale.
class LeadStatusConflictAcknowledged extends LeadDetailEvent {
  const LeadStatusConflictAcknowledged();
}

class LeadNoteSubmitted extends LeadDetailEvent {
  final String body;

  const LeadNoteSubmitted(this.body);

  @override
  List<Object?> get props => [body];
}

/// No phone param — the bloc already has the loaded `Lead.phone`, the
/// single source of truth for what gets dialed (acceptance criterion #6,
/// #72).
class LeadCallRequested extends LeadDetailEvent {
  const LeadCallRequested();
}

/// No phone param — uses the loaded `Lead.phoneE164`. The button that
/// dispatches this is only ever enabled when that field is non-null; see
/// `LeadDetailBloc._onWhatsAppRequested`'s doc comment for what happens
/// if it's dispatched anyway.
class LeadWhatsAppRequested extends LeadDetailEvent {
  const LeadWhatsAppRequested();
}
