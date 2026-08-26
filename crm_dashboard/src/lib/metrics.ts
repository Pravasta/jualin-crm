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
