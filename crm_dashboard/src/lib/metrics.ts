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
import type { LeadSource, LeadStatus, LostReason } from "./labels";

export interface MetricsSummary {
  total_new: number;
  by_status: Partial<Record<LeadStatus, number>>;
  unassigned: number;
  conversion_rate: number | null;
}

export interface MetricsSummaryFilter {
  createdFrom?: string;
  createdTo?: string;
  /** Membership id — narrows to one member (Phase 8.6 #169). */
  assignedTo?: string;
  source?: LeadSource;
}

// One query string for every /v1/metrics/* call, so the Laporan screen's
// filter row means the same thing to all eight blocks (api.md "Filter
// bersama").
function metricsQuery(filter: MetricsSummaryFilter): string {
  const params = new URLSearchParams();
  if (filter.createdFrom) params.set("from", filter.createdFrom);
  if (filter.createdTo) params.set("to", filter.createdTo);
  if (filter.assignedTo) params.set("assigned_to", filter.assignedTo);
  if (filter.source) params.set("source", filter.source);
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

export function getMetricsSummary(
  filter: MetricsSummaryFilter = {},
  signal?: AbortSignal
): Promise<MetricsSummary> {
  return apiFetch<MetricsSummary>(`/v1/metrics/summary${metricsQuery(filter)}`, { signal });
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
  return apiFetch<EmployeeMetric[]>(`/v1/metrics/employees${metricsQuery(filter)}`, { signal });
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
  // Rounding 20 seconds to "0 menit" read as "responds instantly" — a
  // real Beranda row showed it (#163). Under a minute says so.
  if (seconds < 60) return "< 1 menit";
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

// --- Laporan (Phase 8.6 #171) — shapes verified against
// crm_be/internal/metrics/handler_http.go and docs/architecture/api.md.

export interface MetricsTrend {
  bucket: "day" | "week";
  /** `date` is a calendar date (YYYY-MM-DD) in the organization's timezone,
   *  not an instant — never pass it through `new Date()` as-is. */
  points: { date: string; count: number }[];
}

/** from and to are REQUIRED here (400 otherwise). */
export function getMetricsTrend(filter: MetricsSummaryFilter, signal?: AbortSignal): Promise<MetricsTrend> {
  return apiFetch<MetricsTrend>(`/v1/metrics/trend${metricsQuery(filter)}`, { signal });
}

export interface SourceMetric {
  source: LeadSource;
  count: number;
  won_count: number;
  conversion_rate: number | null;
}

export function getMetricsSources(filter: MetricsSummaryFilter, signal?: AbortSignal): Promise<SourceMetric[]> {
  return apiFetch<SourceMetric[]>(`/v1/metrics/sources${metricsQuery(filter)}`, { signal });
}

export interface LostReasonMetric {
  reason: LostReason;
  count: number;
}

export function getMetricsLostReasons(filter: MetricsSummaryFilter, signal?: AbortSignal): Promise<LostReasonMetric[]> {
  return apiFetch<LostReasonMetric[]>(`/v1/metrics/lost-reasons${metricsQuery(filter)}`, { signal });
}

export type ResponseBucket = "lt_1h" | "1h_4h" | "4h_24h" | "gt_24h" | "never_touched";

export interface MetricsResponseTimes {
  buckets: { bucket: ResponseBucket; count: number }[];
  median_seconds: number | null;
}

export const RESPONSE_BUCKET_LABELS: Record<ResponseBucket, string> = {
  lt_1h: "< 1 jam",
  "1h_4h": "1–4 jam",
  "4h_24h": "4–24 jam",
  gt_24h: "> 1 hari",
  never_touched: "Belum disentuh",
};

export function getMetricsResponseTimes(
  filter: MetricsSummaryFilter,
  signal?: AbortSignal
): Promise<MetricsResponseTimes> {
  return apiFetch<MetricsResponseTimes>(`/v1/metrics/response-times${metricsQuery(filter)}`, { signal });
}

export interface TaskMetric {
  membership_id: string;
  full_name: string;
  completed_count: number;
  /** Open and past due RIGHT NOW — ignores the period (api.md). */
  overdue_count: number;
}

export function getMetricsTasks(filter: MetricsSummaryFilter, signal?: AbortSignal): Promise<TaskMetric[]> {
  return apiFetch<TaskMetric[]>(`/v1/metrics/tasks${metricsQuery(filter)}`, { signal });
}
