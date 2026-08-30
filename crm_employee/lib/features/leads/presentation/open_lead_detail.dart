import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../core/di/injection_container.dart';
import 'bloc/lead_detail_bloc.dart';
import 'bloc/lead_detail_event.dart';
import 'pages/lead_detail_page.dart';

/// Push, providing a fresh `LeadDetailBloc` for this one visit and
/// dispatching its initial load. Extracted from `leads_page.dart`'s own
/// private `_openLeadDetail` (#71/#72) once #73 needed the SAME push
/// from three more places — Tugas Saya (tap a task), Notifikasi (tap a
/// notification), and push deeplink (tap/open from a system
/// notification) — the second-plus real usage Aturan #28 asks for
/// before extracting.
void openLeadDetail(BuildContext context, String leadId) {
  Navigator.of(context).push(
    MaterialPageRoute<void>(
      builder: (_) => BlocProvider<LeadDetailBloc>(
        create: (_) => sl<LeadDetailBloc>()..add(LeadDetailRequested(leadId)),
        child: const LeadDetailPage(),
      ),
    ),
  );
}
