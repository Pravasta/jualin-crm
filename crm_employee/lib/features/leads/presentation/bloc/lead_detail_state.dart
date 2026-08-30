import 'package:equatable/equatable.dart';

import '../../domain/entities/activity.dart';
import '../../domain/entities/lead.dart';

sealed class LeadDetailState extends Equatable {
  const LeadDetailState();

  @override
  List<Object?> get props => [];
}

class LeadDetailInitial extends LeadDetailState {
  const LeadDetailInitial();
}

class LeadDetailLoading extends LeadDetailState {
  const LeadDetailLoading();
}

/// The one non-terminal state — everything the screen can do (change
/// status, add a note, call, WhatsApp) is a transient sub-state layered
/// on top of an already-loaded [lead]/[activities], never a separate
/// top-level state. That keeps the lead/timeline on screen throughout —
/// e.g. the conflict dialog (design brief §8.2) floats over the last-
/// known-good content, it doesn't replace it.
class LeadDetailLoaded extends LeadDetailState {
  final Lead lead;
  final List<Activity> activities;

  /// TD §7 — true when either the lead or its activities came from the
  /// offline cache (design brief §10: "Dari cache, tanpa sinyal" applies
  /// to Detail Lead too, not just Lead Saya).
  final bool fromCache;
  final DateTime? fetchedAt;

  /// Status-change sub-state.
  final bool isUpdatingStatus;
  final String? statusError;

  /// Non-null while the "muat ulang" dialog should show — the row's
  /// current server-side state from the `409 version_conflict` body
  /// (Aturan #35). Cleared by `LeadStatusConflictAcknowledged`, which
  /// also triggers a full reload — this field is never applied over
  /// [lead] directly; the reload is what makes the screen honest again.
  final Lead? conflict;

  /// Note-form sub-state.
  final bool isSubmittingNote;
  final String? noteError;

  /// Shared by "Telepon" and "WhatsApp" — design brief §8.3: an activity
  /// is only ever logged AFTER the OS confirms handoff, and both actions
  /// follow the identical launch-then-log shape, so one pair of flags
  /// covers both (only one of the two buttons is reachable at a time in
  /// practice, since both live in the same bottom action bar).
  final bool isLaunchingExternalAction;
  final String? externalActionError;

  const LeadDetailLoaded({
    required this.lead,
    required this.activities,
    required this.fromCache,
    this.fetchedAt,
    this.isUpdatingStatus = false,
    this.statusError,
    this.conflict,
    this.isSubmittingNote = false,
    this.noteError,
    this.isLaunchingExternalAction = false,
    this.externalActionError,
  });

  @override
  List<Object?> get props => [
    lead,
    activities,
    fromCache,
    fetchedAt,
    isUpdatingStatus,
    statusError,
    conflict,
    isSubmittingNote,
    noteError,
    isLaunchingExternalAction,
    externalActionError,
  ];
}

/// Both `getLeadDetail` and `getLeadActivities` failing (or either
/// failing on the very first load, before there's any last-known-good
/// content to keep showing) lands here — Detail Lead needs both pieces
/// to be useful, so a partial screen would be worse than a clear,
/// retry-able error (same stance `LeadsBloc` takes: no partial-success
/// concept there either). Carries [leadId] — unlike `LeadsError` (which
/// can always re-derive its request from `LeadsState`'s own filter/query
/// fields), there is nothing else in this state tree to recover which
/// lead a retry should reload once the last-known-good `Lead` is gone.
class LeadDetailError extends LeadDetailState {
  final String leadId;
  final String message;

  const LeadDetailError(this.leadId, this.message);

  @override
  List<Object?> get props => [leadId, message];
}
