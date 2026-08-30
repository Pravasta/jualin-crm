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

/// `crm_be` allows `lost` to reopen to ANY main-path status (a
/// documented simplification — not implementable as "one step back to
/// whatever it was" without activity history, `crm_be` issue #20's
/// notes). Offering all five as buttons would be a wall of options for a
/// case that's actually rare; `lost` → `new` only is kept (same choice
/// `lead-status.ts` already made) — unconditionally valid per the rule
/// above.
List<StatusTransitionOption> statusTransitionOptions(String from) {
  if (from == 'lost') {
    return [
      StatusTransitionOption(
        status: 'new',
        label: '→ Buka kembali ke ${statusMeta['new']!.label}',
        kind: StatusTransitionKind.step,
      ),
    ];
  }

  final options = <StatusTransitionOption>[];
  final idx = _mainPathIndex(from);
  if (idx != -1) {
    if (idx - 1 >= 0) {
      final prev = _mainPath[idx - 1];
      options.add(
        StatusTransitionOption(
          status: prev,
          label: '→ ${statusMeta[prev]!.label}',
          kind: StatusTransitionKind.step,
        ),
      );
    }
    if (idx + 1 < _mainPath.length) {
      final next = _mainPath[idx + 1];
      options.add(
        StatusTransitionOption(
          status: next,
          label: '→ ${statusMeta[next]!.label}',
          kind: StatusTransitionKind.step,
        ),
      );
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
