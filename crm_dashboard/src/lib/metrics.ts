// Typed wrapper for GET /v1/metrics/summary — shape verified against
// crm_be/internal/metrics/handler_http.go's summaryJSON.
//
// Used here only for the status/unassigned chip counts on the lead list
// (and the sidebar's unassigned badge). Those counts are org-wide for
// the selected period — NOT narrowed by source/owner/keyword, since the
// endpoint doesn't support that combination. That's a deliberate
// simplification (see notes.md "## #32"), not a bug: getting an exact
// per-combination count would mean one request per chip.
import { apiFetch } from "./api-client";
import type { LeadStatus } from "./labels";

export interface MetricsSummary {
  total_new: number;
  by_status: Partial<Record<LeadStatus, number>>;
  unassigned: number;
  conversion_rate: number | null;
}

export interface MetricsSummaryFilter {
  createdFrom?: string;
  createdTo?: string;
}

export function getMetricsSummary(
  filter: MetricsSummaryFilter = {},
  signal?: AbortSignal
): Promise<MetricsSummary> {
  const params = new URLSearchParams();
  if (filter.createdFrom) params.set("from", filter.createdFrom);
  if (filter.createdTo) params.set("to", filter.createdTo);
  const qs = params.toString();
  return apiFetch<MetricsSummary>(`/v1/metrics/summary${qs ? `?${qs}` : ""}`, { signal });
}

// Shape verified against crm_be/internal/metrics/handler_http.go's
// employeeJSON — despite the name, one row per MEMBERSHIP with an
// assigned lead in range, not restricted to role=employee (Owner/Admin/
// Manager can hold assignments too).
export interface EmployeeMetric {
  membership_id: string;
  full_name: string;
  lead_count: number;
  /** null when this member has no assigned lead in range ever touched
   *  by an activity — excluded from the average, not a real zero. */
  avg_response_seconds: number | null;
  converted_count: number;
}

export function getMetricsEmployees(
  filter: MetricsSummaryFilter = {},
  signal?: AbortSignal
): Promise<EmployeeMetric[]> {
  const params = new URLSearchParams();
  if (filter.createdFrom) params.set("from", filter.createdFrom);
  if (filter.createdTo) params.set("to", filter.createdTo);
  const qs = params.toString();
  return apiFetch<EmployeeMetric[]>(`/v1/metrics/employees${qs ? `?${qs}` : ""}`, { signal });
}

// conversion_rate is a raw fraction (won / (total - spam - unqualified)),
// NOT a percentage — internal/metrics/repository_postgres.go's Summary
// divides two counts directly, no ×100. null means the denominator was
// zero ("belum ada data", not "0%" — issue #35's AC and TD §2.2 are both
// explicit this must render differently from a real zero).
export function formatConversionRate(rate: number | null): string {
  if (rate === null) return "Belum ada data";
  return `${Math.round(rate * 100)}%`;
}

// avg_response_seconds is null (not 0) when nobody's touched an
// assigned lead yet in range — same "no data" distinction as
// conversion_rate, so it gets the same "—" treatment rather than "0
// menit" which would read as "responds instantly".
export function formatAvgResponseSeconds(seconds: number | null): string {
  if (seconds === null) return "—";
  if (seconds < 3600) return `${Math.round(seconds / 60)} menit`;
  return `${Math.round(seconds / 3600)} jam`;
}

export type MetricsPeriod = "7d" | "30d" | "90d";

export const METRICS_PERIODS: { value: MetricsPeriod; label: string }[] = [
  { value: "7d", label: "7 hari terakhir" },
  { value: "30d", label: "30 hari terakhir" },
  { value: "90d", label: "90 hari terakhir" },
];

// `now` is a parameter (never `new Date()` computed internally) so this
// stays a pure, testable function — the caller supplies "now" once per
// render instead of this function reaching for global time itself.
export function periodToRange(period: MetricsPeriod, now: Date): { from: string; to: string } {
  const days = { "7d": 7, "30d": 30, "90d": 90 }[period];
  const from = new Date(now);
  from.setUTCDate(from.getUTCDate() - days);
  return { from: from.toISOString(), to: now.toISOString() };
}

// by_status is a Go map — a status with zero leads is OMITTED from the
// JSON entirely (verified against a real crm_be), not sent as 0.
// `summary.by_status[status] ?? "…"` would show the loading placeholder
// forever for any status nobody has ever used. "…" is correct ONLY while
// summary itself hasn't loaded yet; once it has, a missing key means 0.
export function statusCount(
  summary: MetricsSummary | null,
  status: LeadStatus
): number | "…" {
  if (!summary) return "…";
  return summary.by_status[status] ?? 0;
}
