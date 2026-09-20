import '../../../shared/labels.dart';

/// Which status transitions the picker offers. Mirrors
/// `crm_be/internal/lead/usecase.go`'s `validateStatusTransition`
/// EXACTLY — copied from `crm_dashboard/src/lib/lead-status.ts`'s own
/// line-for-line port of the same Go function (#33), not from the
/// Claude Design mockup's simplified transition set. Acceptance
/// criterion #72: "transisi yang ditawarkan UI tidak pernah memuat yang
/// backend tolak" — a button the backend then rejects is a worse
/// failure than one the UI never shows.
const List<String> _mainPath = [
  'new',
  'contacted',
  'qualified',
  'proposal',
  'won',
];
const List<String> _sideExits = ['lost', 'unqualified', 'spam'];

int _mainPathIndex(String status) => _mainPath.indexOf(status);

/// Line-for-line port of the Go function of the same name.
bool isValidStatusTransition(String from, String to) {
  if (from == 'unqualified' || from == 'spam') return false;
  if (to == from) return to == 'lost';
  // Nothing leads back to 'new' (issue #139, ADR-015): it means "belum
  // disentuh", which can never be true again once a lead has been touched.
  if (to == 'new') return false;
  if (to == 'unqualified' || to == 'spam' || to == 'lost') return true;

  final toIdx = _mainPathIndex(to);
  if (toIdx == -1) return false;
  if (from == 'lost') return true;

  final fromIdx = _mainPathIndex(from);
  if (fromIdx == -1) return false;
  final diff = toIdx - fromIdx;
  return diff == 1 || diff == -1;
}

enum StatusTransitionKind { step, exit }

class StatusTransitionOption {
  final String status;
  final String label;

  /// `step` = adjacent on the main path; `exit` = a side terminal.
  final StatusTransitionKind kind;

  const StatusTransitionOption({
    required this.status,
    required this.label,
    required this.kind,
  });
}

/// `crm_be` allows `lost` to reopen to ANY main-path status except `new`
/// (a documented simplification — not implementable as "one step back to
/// whatever it was" without activity history, `crm_be` issue #20's
/// notes). Offering all four as buttons would be a wall of options for a
/// case that's actually rare; `lost` → `contacted` only is offered (same
/// choice `lead-status.ts` made) — the earliest stage a reopened lead can
/// honestly be in now that `new` is closed (ADR-015), and valid per the
/// rule above.
List<StatusTransitionOption> statusTransitionOptions(String from) {
  if (from == 'lost') {
    return [
      StatusTransitionOption(
        status: 'contacted',
        label: '→ Buka kembali ke ${statusMeta['contacted']!.label}',
        kind: StatusTransitionKind.step,
      ),
    ];
  }

  final options = <StatusTransitionOption>[];
  final idx = _mainPathIndex(from);
  if (idx != -1) {
    // Neighbors are offered only if the rule says so — this is what keeps
    // 'contacted' from showing a way back to 'new'. Deriving them from the
    // path index alone is how that button appeared in the first place.
    if (idx - 1 >= 0) {
      final prev = _mainPath[idx - 1];
      if (isValidStatusTransition(from, prev)) {
        options.add(
          StatusTransitionOption(
            status: prev,
            label: '→ ${statusMeta[prev]!.label}',
            kind: StatusTransitionKind.step,
          ),
        );
      }
    }
    if (idx + 1 < _mainPath.length) {
      final next = _mainPath[idx + 1];
      if (isValidStatusTransition(from, next)) {
        options.add(
          StatusTransitionOption(
            status: next,
            label: '→ ${statusMeta[next]!.label}',
            kind: StatusTransitionKind.step,
          ),
        );
      }
    }
  }

  for (final exit in _sideExits) {
    if (isValidStatusTransition(from, exit)) {
      options.add(
        StatusTransitionOption(
          status: exit,
          label: statusMeta[exit]!.label,
          kind: StatusTransitionKind.exit,
        ),
      );
    }
  }
  return options;
}
