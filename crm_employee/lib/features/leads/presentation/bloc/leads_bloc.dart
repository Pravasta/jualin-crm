import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/error/failures.dart';
import '../../../auth/presentation/bloc/auth_bloc.dart';
import '../../../auth/presentation/bloc/auth_event.dart';
import '../../domain/usecases/get_my_leads_usecase.dart';
import 'leads_event.dart';
import 'leads_state.dart';

class LeadsBloc extends Bloc<LeadsEvent, LeadsState> {
  final GetMyLeadsUseCase getMyLeads;

  /// Not imported for its own screens — only to dispatch
  /// `AuthSessionInvalidated` when this bloc's own call surfaces
  /// `SessionExpiredFailure` (see that event's doc comment). Presentation
  /// coordinating with presentation; `LeadRepositoryImpl`/`GetMyLeadsUseCase`
  /// never know `AuthBloc` exists.
  final AuthBloc authBloc;

  LeadsBloc({required this.getMyLeads, required this.authBloc})
    : super(const LeadsInitial()) {
    on<LeadsRequested>(_onRequested);
    on<LeadsStatusFilterChanged>(_onStatusFilterChanged);
    on<LeadsSearchChanged>(_onSearchChanged);
    on<LeadsRefreshRequested>(_onRefreshRequested);
  }

  Future<void> _onRequested(
    LeadsRequested event,
    Emitter<LeadsState> emit,
  ) async {
    await _load(emit, status: state.statusFilter, query: state.query);
  }

  Future<void> _onStatusFilterChanged(
    LeadsStatusFilterChanged event,
    Emitter<LeadsState> emit,
  ) async {
    await _load(emit, status: event.status, query: state.query);
  }

  Future<void> _onSearchChanged(
    LeadsSearchChanged event,
    Emitter<LeadsState> emit,
  ) async {
    await _load(emit, status: state.statusFilter, query: event.query);
  }

  Future<void> _onRefreshRequested(
    LeadsRefreshRequested event,
    Emitter<LeadsState> emit,
  ) async {
    await _load(emit, status: state.statusFilter, query: state.query);
  }

  Future<void> _load(
    Emitter<LeadsState> emit, {
    required String? status,
    required String query,
  }) async {
    emit(LeadsLoading(statusFilter: status, query: query));

    final result = await getMyLeads(
      GetMyLeadsParams(status: status, query: query.isEmpty ? null : query),
    );

    result.fold(
      (failure) {
        if (failure is SessionExpiredFailure) {
          authBloc.add(const AuthSessionInvalidated());
          return;
        }
        emit(LeadsError(failure.message, statusFilter: status, query: query));
      },
      (leadList) => emit(
        LeadsLoaded(
          leads: leadList.leads,
          total: leadList.total,
          fromCache: leadList.fromCache,
          fetchedAt: leadList.fetchedAt,
          statusFilter: status,
          query: query,
        ),
      ),
    );
  }
}
