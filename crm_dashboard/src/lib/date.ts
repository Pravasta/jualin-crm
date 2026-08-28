// `<input type="date">` gives a bare "YYYY-MM-DD" with no timezone.
// crm_be's created_from/created_to are ISO 8601 UTC (Aturan #33) and
// filter leads.created_at directly — so the FROM bound must be the
// start of that day and the TO bound the end of it, or a lead created
// on the last day of the range gets silently excluded.
export function dateInputToStartOfDayUTC(value: string): string | undefined {
  if (!value) return undefined;
  return `${value}T00:00:00.000Z`;
}

export function dateInputToEndOfDayUTC(value: string): string | undefined {
  if (!value) return undefined;
  return `${value}T23:59:59.999Z`;
}

// Same format the design uses (fmtDate): "17 Agu 2026".
export function formatDateID(iso: string): string {
  return new Date(iso).toLocaleDateString("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

// api_keys.last_used_at is throttled server-side to at most once every 5
// minutes (crm_be TD phase 4 §10) — rendering it as an exact timestamp
// would imply a precision the value doesn't actually have. `now` is a
// parameter rather than computed internally (same convention as
// metrics.ts's periodToRange) so this stays pure and testable.
export function formatApproximateID(iso: string, now: Date): string {
  const diffMinutes = Math.max(0, Math.round((now.getTime() - new Date(iso).getTime()) / 60_000));

  if (diffMinutes < 1) return "sekitar baru saja";
  if (diffMinutes < 60) return `sekitar ${diffMinutes} menit lalu`;

  const diffHours = Math.round(diffMinutes / 60);
  if (diffHours < 24) return `sekitar ${diffHours} jam lalu`;

  const diffDays = Math.round(diffHours / 24);
  return `sekitar ${diffDays} hari lalu`;
}
