import 'package:equatable/equatable.dart';

import '../../domain/entities/push_message.dart';

/// Flat, not a sealed hierarchy — unlike `LeadsState`/`AuthState`, `push`
/// has no loading/loaded/error lifecycle to model; it just accumulates
/// two independent, unrelated facts over the app's lifetime (a pending
/// deeplink target, a foreground banner to show), so one class with two
/// nullable fields is the honest shape, not four subclasses covering
/// their cross product.
class PushState extends Equatable {
  /// Set when a push was tapped (background or cold start) — the id of
  /// the lead to navigate to, once `AuthBloc` is `AuthAuthenticated`.
  /// Survives across a login that hasn't happened yet (TD §10's
  /// required case: tapped while logged out).
  final String? pendingLeadId;

  /// Set when a push arrives while the app is already open — design
  /// brief §10: an in-app banner, never a forced navigation. Cleared by
  /// `PushForegroundBannerDismissed` or by tapping the banner itself
  /// (which routes through `PushMessageTapped` the same as any other
  /// tap, setting `pendingLeadId` instead).
  final PushMessage? foregroundMessage;

  const PushState({this.pendingLeadId, this.foregroundMessage});

  @override
  List<Object?> get props => [pendingLeadId, foregroundMessage];
}
