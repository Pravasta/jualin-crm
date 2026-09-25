// Laporan's period filter (Phase 8.6 #171, brief §10.2): 7 hari, 30 hari,
// bulan ini, bulan lalu, or a custom range.
//
// Bounds are LOCAL calendar days, turned into instants. /v1/metrics/trend
// buckets by the organization's calendar days and returns every day the
// range touches (api.md, #169) — so a range sent as UTC midnights shows an
// extra day for a WIB organization. The dashboard doesn't receive the
// organization's timezone (/v1/me doesn't carry it), so the browser's is
// the proxy: for an Indonesian business looking at its own dashboard they
// are the same, and when they differ the chart shows one extra edge day
// rather than wrong numbers. Recorded in notes.md "## #171".

export type ReportPreset = "7d" | "30d" | "month" | "last_month" | "custom";

export const REPORT_PRESETS: { value: ReportPreset; label: string }[] = [
  { value: "7d", label: "7 hari terakhir" },
  { value: "30d", label: "30 hari terakhir" },
  { value: "month", label: "Bulan ini" },
  { value: "last_month", label: "Bulan lalu" },
  { value: "custom", label: "Rentang kustom" },
];

export interface ReportRange {
  /** Instants, ISO 8601 — what /v1/metrics/* takes. */
  from: string;
  to: string;
  /** The same bounds as local calendar dates (YYYY-MM-DD) — what the lead
   *  list's created_from/created_to take, for the "open this list" links. */
  fromDate: string;
  toDate: string;
}

const MAX_DAYS = 366; // the API's own cap on /trend (api.md)

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate(), 0, 0, 0, 0);
}

function endOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate(), 23, 59, 59, 999);
}

function dateInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function parseDateInput(value: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!m) return null;
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  return d.getMonth() === Number(m[2]) - 1 ? d : null; // rejects 2026-02-31
}

function range(from: Date, to: Date): ReportRange {
  const start = startOfDay(from);
  const end = endOfDay(to);
  return { from: start.toISOString(), to: end.toISOString(), fromDate: dateInput(start), toDate: dateInput(end) };
}

/**
 * The range a preset means on `now` (injectable, so tests don't read the
 * clock). "7 hari terakhir" is today and the six days before it — seven
 * calendar days, not 168 hours back from this minute.
 */
export function presetRange(preset: Exclude<ReportPreset, "custom">, now: Date): ReportRange {
  const today = startOfDay(now);
  switch (preset) {
    case "7d":
      return range(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 6), today);
    case "30d":
      return range(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 29), today);
    case "month":
      return range(new Date(today.getFullYear(), today.getMonth(), 1), today);
    case "last_month":
      // Day 0 of this month is the last day of the previous one.
      return range(new Date(today.getFullYear(), today.getMonth() - 1, 1), new Date(today.getFullYear(), today.getMonth(), 0));
  }
}

/**
 * A custom range, or why it can't be used. Checked here rather than left to
 * the API's 400: the screen can say which field is wrong before any of the
 * eight blocks tries to load.
 */
export function customRange(fromValue: string, toValue: string): { range: ReportRange } | { problem: string } {
  const from = parseDateInput(fromValue);
  const to = parseDateInput(toValue);
  if (!from || !to) return { problem: "Isi tanggal mulai dan tanggal akhir." };
  if (to < from) return { problem: "Tanggal akhir tidak boleh sebelum tanggal mulai." };
  const days = Math.round((to.getTime() - from.getTime()) / 86_400_000) + 1;
  if (days > MAX_DAYS) return { problem: `Rentang paling panjang ${MAX_DAYS} hari.` };
  return { range: range(from, to) };
}
