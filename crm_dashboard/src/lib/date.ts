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
