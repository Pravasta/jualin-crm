import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../../../auth/presentation/bloc/auth_bloc.dart';
import '../../../auth/presentation/bloc/auth_event.dart';
import '../../domain/entities/notification.dart';
import '../../domain/usecases/get_notifications_usecase.dart';
import '../../domain/usecases/mark_notification_read_usecase.dart';
import 'notifications_event.dart';
import 'notifications_state.dart';

class NotificationsBloc extends Bloc<NotificationsEvent, NotificationsState> {
  final GetNotificationsUseCase getNotifications;
  final MarkNotificationReadUseCase markNotificationRead;

  /// Same reasoning as `LeadsBloc.authBloc` — only to dispatch
  /// `AuthSessionInvalidated`.
  final AuthBloc authBloc;

  NotificationsBloc({
    required this.getNotifications,
    required this.markNotificationRead,
    required this.authBloc,
  }) : super(const NotificationsInitial()) {
    on<NotificationsRequested>(_onRequested);
    on<NotificationsRefreshRequested>(_onRefreshRequested);
    on<NotificationMarkReadRequested>(_onMarkReadRequested);
  }

  Future<void> _onRequested(
    NotificationsRequested event,
    Emitter<NotificationsState> emit,
  ) async {
    await _load(emit);
  }

  Future<void> _onRefreshRequested(
    NotificationsRefreshRequested event,
    Emitter<NotificationsState> emit,
  ) async {
    await _load(emit);
  }

  Future<void> _load(Emitter<NotificationsState> emit) async {
    emit(const NotificationsLoading());

    final result = await getNotifications(const NoParams());

    result.fold((failure) {
      if (failure is SessionExpiredFailure) {
        authBloc.add(const AuthSessionInvalidated());
        return;
      }
      emit(NotificationsError(failure.message));
    }, (list) => emit(NotificationsLoaded(list.notifications)));
  }

  Future<void> _onMarkReadRequested(
    NotificationMarkReadRequested event,
    Emitter<NotificationsState> emit,
  ) async {
    final current = state;
    if (current is! NotificationsLoaded) return;

    // Optimistic — see NotificationMarkReadRequested's doc comment.
    final now = DateTime.now();
    emit(
      NotificationsLoaded([
        for (final n in current.notifications)
          if (n.id == event.id)
            NotificationItem(
              id: n.id,
              type: n.type,
              leadId: n.leadId,
              taskId: n.taskId,
              title: n.title,
              body: n.body,
              readAt: n.readAt ?? now,
              createdAt: n.createdAt,
            )
          else
            n,
      ]),
    );

    final result = await markNotificationRead(event.id);
    result.fold((failure) {
      if (failure is SessionExpiredFailure) {
        authBloc.add(const AuthSessionInvalidated());
      }
      // Any other failure: the optimistic update stands. A row that
      // shows read locally but wasn't actually marked server-side is a
      // harmless, self-correcting inconsistency (the next full reload
      // reflects the truth) — not worth an error banner over.
    }, (_) {});
  }
}
