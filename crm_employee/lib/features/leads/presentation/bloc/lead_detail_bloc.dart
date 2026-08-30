import 'package:dartz/dartz.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/failures.dart';
import '../../../auth/presentation/bloc/auth_bloc.dart';
import '../../../auth/presentation/bloc/auth_event.dart';
import '../../domain/entities/activity.dart';
import '../../domain/entities/lead.dart';
import '../../domain/usecases/add_lead_note_usecase.dart';
import '../../domain/usecases/get_lead_activities_usecase.dart';
import '../../domain/usecases/get_lead_detail_usecase.dart';
import '../../domain/usecases/launch_dialer_usecase.dart';
import '../../domain/usecases/launch_whatsapp_usecase.dart';
import '../../domain/usecases/log_call_usecase.dart';
import '../../domain/usecases/log_whatsapp_opened_usecase.dart';
import '../../domain/usecases/update_lead_status_usecase.dart';
import 'lead_detail_event.dart';
import 'lead_detail_state.dart';

class LeadDetailBloc extends Bloc<LeadDetailEvent, LeadDetailState> {
  final GetLeadDetailUseCase getLeadDetail;
  final GetLeadActivitiesUseCase getLeadActivities;
  final UpdateLeadStatusUseCase updateLeadStatus;
  final AddLeadNoteUseCase addLeadNote;
  final LogCallUseCase logCall;
  final LogWhatsAppOpenedUseCase logWhatsAppOpened;
  final LaunchDialerUseCase launchDialer;
  final LaunchWhatsAppUseCase launchWhatsApp;

  /// Same reasoning as `LeadsBloc.authBloc` — only to dispatch
  /// `AuthSessionInvalidated` when a call here surfaces
  /// `SessionExpiredFailure`. Presentation coordinating with
  /// presentation; the use cases above never know `AuthBloc` exists.
  final AuthBloc authBloc;

  LeadDetailBloc({
    required this.getLeadDetail,
    required this.getLeadActivities,
    required this.updateLeadStatus,
    required this.addLeadNote,
    required this.logCall,
    required this.logWhatsAppOpened,
    required this.launchDialer,
    required this.launchWhatsApp,
    required this.authBloc,
  }) : super(const LeadDetailInitial()) {
    on<LeadDetailRequested>(_onRequested);
    on<LeadDetailRefreshRequested>(_onRefreshRequested);
    on<LeadStatusChangeRequested>(_onStatusChangeRequested);
    on<LeadStatusConflictAcknowledged>(_onConflictAcknowledged);
    on<LeadNoteSubmitted>(_onNoteSubmitted);
    on<LeadCallRequested>(_onCallRequested);
    on<LeadWhatsAppRequested>(_onWhatsAppRequested);
  }

  Future<void> _onRequested(
    LeadDetailRequested event,
    Emitter<LeadDetailState> emit,
  ) async {
    await _load(event.leadId, emit);
  }

  Future<void> _onRefreshRequested(
    LeadDetailRefreshRequested event,
    Emitter<LeadDetailState> emit,
  ) async {
    final current = state;
    if (current is! LeadDetailLoaded) return;
    await _load(current.lead.id, emit);
  }

  /// Used for the initial load and for an explicit pull-to-refresh only
  /// — both blank the screen to [LeadDetailLoading] first, same trade-off
  /// `LeadsBloc._load` already accepted for its own refresh gesture. A
  /// write action (status/note/call/WhatsApp) never calls this — those
  /// update in place via [_withFreshActivities] so the lead/timeline stay
  /// on screen throughout.
  Future<void> _load(String leadId, Emitter<LeadDetailState> emit) async {
    emit(const LeadDetailLoading());

    // Both requests start before either is awaited — genuinely
    // concurrent, not sequential round-trips.
    final leadFuture = getLeadDetail(leadId);
    final activitiesFuture = getLeadActivities(leadId);
    final leadResult = await leadFuture;
    final activitiesResult = await activitiesFuture;

    // Detail Lead needs both pieces to be useful — a lead with a broken
    // timeline (or vice versa) is worse than one clear, retry-able error
    // screen (see LeadDetailError's doc comment).
    Failure? sessionFailure;
    Failure? otherFailure;
    LeadDetailResult? leadDetail;
    ActivityListResult? activityList;

    leadResult.fold((failure) {
      if (failure is SessionExpiredFailure) {
        sessionFailure = failure;
      } else {
        otherFailure ??= failure;
      }
    }, (value) => leadDetail = value);

    activitiesResult.fold((failure) {
      if (failure is SessionExpiredFailure) {
        sessionFailure = failure;
      } else {
        otherFailure ??= failure;
      }
    }, (value) => activityList = value);

    if (sessionFailure != null) {
      authBloc.add(const AuthSessionInvalidated());
      return;
    }
    if (otherFailure != null) {
      emit(LeadDetailError(leadId, otherFailure!.message));
      return;
    }

    emit(
      LeadDetailLoaded(
        lead: leadDetail!.lead,
        activities: activityList!.activities,
        fromCache: leadDetail!.fromCache || activityList!.fromCache,
        fetchedAt: leadDetail!.fetchedAt ?? activityList!.fetchedAt,
      ),
    );
  }

  Future<void> _onStatusChangeRequested(
    LeadStatusChangeRequested event,
    Emitter<LeadDetailState> emit,
  ) async {
    final current = state;
    if (current is! LeadDetailLoaded) return;

    emit(_transient(current, isUpdatingStatus: true));

    final result = await updateLeadStatus(
      UpdateLeadStatusParams(
        leadId: current.lead.id,
        version: current.lead.version,
        status: event.status,
        lostReason: event.lostReason,
      ),
    );

    await result.fold(
      (failure) async {
        if (failure is SessionExpiredFailure) {
          authBloc.add(const AuthSessionInvalidated());
          return;
        }
        if (failure is VersionConflictFailure<Lead>) {
          // Design brief §8.2 — never silently overwritten. The dialog
          // reads [conflict]; "muat ulang" is the only way out
          // (LeadStatusConflictAcknowledged), never a retry of this same
          // write.
          emit(_transient(current, conflict: failure.current));
          return;
        }
        emit(_transient(current, statusError: failure.message));
      },
      (updatedLead) async {
        // The status change itself succeeded — crm_be also recorded a
        // status_changed activity server-side, so refetch rather than
        // fabricate that entry client-side.
        final refreshed = await _withFreshActivities(
          lead: updatedLead,
          previousActivities: current.activities,
          previousFromCache: false,
          previousFetchedAt: null,
        );
        if (refreshed != null) emit(refreshed);
      },
    );
  }

  Future<void> _onConflictAcknowledged(
    LeadStatusConflictAcknowledged event,
    Emitter<LeadDetailState> emit,
  ) async {
    final current = state;
    if (current is! LeadDetailLoaded) return;
    // "Muat ulang" — a genuine reload, not a retry of the rejected write.
    await _load(current.lead.id, emit);
  }

  Future<void> _onNoteSubmitted(
    LeadNoteSubmitted event,
    Emitter<LeadDetailState> emit,
  ) async {
    final current = state;
    if (current is! LeadDetailLoaded) return;

    final body = event.body.trim();
    if (body.isEmpty) {
      emit(_transient(current, noteError: 'Catatan tidak boleh kosong.'));
      return;
    }

    emit(_transient(current, isSubmittingNote: true));

    final result = await addLeadNote(
      AddLeadNoteParams(leadId: current.lead.id, body: body),
    );

    await result.fold(
      (failure) async {
        if (failure is SessionExpiredFailure) {
          authBloc.add(const AuthSessionInvalidated());
          return;
        }
        emit(_transient(current, noteError: failure.message));
      },
      (_) async {
        final refreshed = await _withFreshActivities(
          lead: current.lead,
          previousActivities: current.activities,
          previousFromCache: current.fromCache,
          previousFetchedAt: current.fetchedAt,
        );
        if (refreshed != null) emit(refreshed);
      },
    );
  }

  Future<void> _onCallRequested(
    LeadCallRequested event,
    Emitter<LeadDetailState> emit,
  ) async {
    final current = state;
    if (current is! LeadDetailLoaded) return;
    final phone = current.lead.phone;
    // Defensive — the button is only ever shown when there's a phone to
    // call at all.
    if (phone == null || phone.isEmpty) return;

    await _launchAndLog(
      current: current,
      emit: emit,
      launch: () => launchDialer(phone),
      log: () => logCall(LogCallParams(leadId: current.lead.id, phone: phone)),
    );
  }

  Future<void> _onWhatsAppRequested(
    LeadWhatsAppRequested event,
    Emitter<LeadDetailState> emit,
  ) async {
    final current = state;
    if (current is! LeadDetailLoaded) return;
    final phoneE164 = current.lead.phoneE164;
    // Defensive — the WhatsApp button is only ever enabled when this is
    // non-null (see Lead.phoneE164's doc comment); this guard exists so
    // a stale dispatch can never reach ExternalActionRepository with a
    // null number, not because the UI is expected to get this wrong.
    if (phoneE164 == null) return;

    await _launchAndLog(
      current: current,
      emit: emit,
      launch: () => launchWhatsApp(phoneE164),
      log: () => logWhatsAppOpened(current.lead.id),
    );
  }

  /// Shared by call and WhatsApp — design brief §8.3: log ONLY after
  /// [launch] confirms the OS actually handed off, never at tap time.
  Future<void> _launchAndLog({
    required LeadDetailLoaded current,
    required Emitter<LeadDetailState> emit,
    required Future<bool> Function() launch,
    required Future<Either<Failure, Activity>> Function() log,
  }) async {
    emit(_transient(current, isLaunchingExternalAction: true));

    final opened = await launch();
    if (!opened) {
      // Canceled before handoff (the OS app-picker/permission prompt) —
      // no activity, and no error either: a deliberate choice, not a
      // failure.
      emit(_transient(current));
      return;
    }

    final result = await log();
    await result.fold(
      (failure) async {
        if (failure is SessionExpiredFailure) {
          authBloc.add(const AuthSessionInvalidated());
          return;
        }
        // The external app DID open — only the activity log failed.
        // Never imply the call/WhatsApp itself didn't happen.
        emit(
          _transient(
            current,
            externalActionError:
                'Aksi berhasil, tapi gagal dicatat: ${failure.message}',
          ),
        );
      },
      (_) async {
        final refreshed = await _withFreshActivities(
          lead: current.lead,
          previousActivities: current.activities,
          previousFromCache: current.fromCache,
          previousFetchedAt: current.fetchedAt,
        );
        if (refreshed != null) emit(refreshed);
      },
    );
  }

  /// Returns null when the session was invalidated (the caller should
  /// stop — `AuthBloc`'s redirect makes any further emit from this bloc
  /// moot). Otherwise: fresh activities layered onto [lead], falling
  /// back to [previousActivities] if the refetch itself fails for a
  /// non-session reason — the write that got us here already succeeded,
  /// and losing that success over a secondary fetch would be worse than
  /// a stale list.
  Future<LeadDetailLoaded?> _withFreshActivities({
    required Lead lead,
    required List<Activity> previousActivities,
    required bool previousFromCache,
    required DateTime? previousFetchedAt,
  }) async {
    final result = await getLeadActivities(lead.id);
    return result.fold((failure) {
      if (failure is SessionExpiredFailure) {
        authBloc.add(const AuthSessionInvalidated());
        return null;
      }
      return LeadDetailLoaded(
        lead: lead,
        activities: previousActivities,
        fromCache: previousFromCache,
        fetchedAt: previousFetchedAt,
      );
    }, (activityResult) => LeadDetailLoaded(lead: lead, activities: activityResult.activities, fromCache: activityResult.fromCache, fetchedAt: activityResult.fetchedAt));
  }

  /// Builds a new [LeadDetailLoaded] carrying [base]'s lead/activities/
  /// cache info forward untouched, with only the transient sub-state
  /// overridden — every field not passed resets to its default (idle),
  /// which is what every call site here wants: dispatching a new action
  /// always clears whatever transient error/flag the previous one left
  /// behind.
  LeadDetailLoaded _transient(
    LeadDetailLoaded base, {
    bool isUpdatingStatus = false,
    String? statusError,
    Lead? conflict,
    bool isSubmittingNote = false,
    String? noteError,
    bool isLaunchingExternalAction = false,
    String? externalActionError,
  }) {
    return LeadDetailLoaded(
      lead: base.lead,
      activities: base.activities,
      fromCache: base.fromCache,
      fetchedAt: base.fetchedAt,
      isUpdatingStatus: isUpdatingStatus,
      statusError: statusError,
      conflict: conflict,
      isSubmittingNote: isSubmittingNote,
      noteError: noteError,
      isLaunchingExternalAction: isLaunchingExternalAction,
      externalActionError: externalActionError,
    );
  }
}
