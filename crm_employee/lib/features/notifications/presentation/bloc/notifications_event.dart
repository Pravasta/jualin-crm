import 'package:equatable/equatable.dart';

sealed class NotificationsEvent extends Equatable {
  const NotificationsEvent();

  @override
  List<Object?> get props => [];
}

class NotificationsRequested extends NotificationsEvent {
  const NotificationsRequested();
}

class NotificationsRefreshRequested extends NotificationsEvent {
  const NotificationsRefreshRequested();
}

/// Dispatched alongside — not instead of — the widget's own navigation
/// (`openLeadDetail`, called directly from the row's `onTap` since the
/// widget already has the item's `leadId`). Updates the row optimistically
/// in local state rather than waiting on/refetching after the network
/// call — `NotificationRepository.markRead`'s own doc comment: this is a
/// low-stakes, fire-and-forget nicety, not a state transition worth the
/// same "pesan inline + refetch" ceremony `TasksBloc` gives task
/// completion.
class NotificationMarkReadRequested extends NotificationsEvent {
  final String id;

  const NotificationMarkReadRequested(this.id);

  @override
  List<Object?> get props => [id];
}
