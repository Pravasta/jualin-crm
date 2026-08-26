"use client";

// Built from the design's HOME section, with the metrics logic ported
// from scratch rather than copied — the mockup computes every number
// (including a fabricated 3-name performance table with a hardcoded
// "(2+i) jam" response time) client-side from an in-memory `s.leads`
// array. AC #10/issue checklist are explicit: metrics come from the
// aggregate endpoints, never computed in the browser from a page of
// data. Only the layout and the null-vs-real-zero treatment for
// conversion rate (the mockup's `isEmpty` idea) carry over.
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Card, CardContent } from "@/components/ui/card";
import { FormErrorBanner } from "@/components/form-error-banner";
import {
  formatAvgResponseSeconds,
  formatConversionRate,
  getMetricsEmployees,
  getMetricsSummary,
  METRICS_PERIODS,
  periodToRange,
  statusCount,
  type EmployeeMetric,
  type MetricsPeriod,
  type MetricsSummary,
} from "@/lib/metrics";
import { LEAD_STATUSES, STATUS_META } from "@/lib/labels";
import { globalMessage } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

function StatCard({
  label,
  value,
  isEmpty,
  valueColor,
  href,
}: {
  label: string;
  value: string;
  isEmpty?: boolean;
  valueColor?: string;
  href?: string;
}) {
  const content = (
    <>
      <div className="mb-2 text-xs text-muted-foreground">{label}</div>
      <div
        className={isEmpty ? "text-[15px] font-medium italic text-muted-foreground" : "text-2xl font-semibold tracking-tight"}
        style={!isEmpty && valueColor ? { color: valueColor } : undefined}
      >
        {value}
      </div>
    </>
  );
  const className = "rounded-[10px] border border-border bg-background p-3.5 text-left";
  if (href) {
    return (
      <Link href={href} className={`${className} block transition-colors hover:bg-muted/40`}>
        {content}
      </Link>
    );
  }
  return <div className={className}>{content}</div>;
}

export function HomeScreen() {
  const session = useSession();
  // ActionMetricsRead excludes Employee (TD §2.4) — "dashboard isn't
  // Employee's tool", they get mobile in Phase 5. Gated here rather than
  // attempting the fetch and rendering a confusing 403.
  const canViewMetrics = session.role !== "employee";

  const [period, setPeriod] = useState<MetricsPeriod>("30d");
  const [summary, setSummary] = useState<MetricsSummary | null>(null);
  const [employees, setEmployees] = useState<EmployeeMetric[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loadedPeriod, setLoadedPeriod] = useState<MetricsPeriod | null>(null);
  const loading = loadedPeriod !== period;

  const range = useMemo(() => periodToRange(period, new Date()), [period]);

  useEffect(() => {
    if (!canViewMetrics) return;
    const controller = new AbortController();
    Promise.all([
      getMetricsSummary({ createdFrom: range.from, createdTo: range.to }, controller.signal),
      getMetricsEmployees({ createdFrom: range.from, createdTo: range.to }, controller.signal),
    ])
      .then(([summaryData, employeeData]) => {
        setSummary(summaryData);
        setEmployees(employeeData);
        setError(null);
        setLoadedPeriod(period);
      })
      .catch((err) => {
        if (isAbortError(err)) return;
        setError(globalMessage(err));
        setLoadedPeriod(period);
      });
    return () => controller.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range.from, range.to, canViewMetrics]);

  if (!canViewMetrics) {
    return (
      <div className="rounded-lg border border-dashed border-muted-foreground/30 bg-background px-6 py-12 text-center text-sm text-muted-foreground">
        Ringkasan metrik tidak tersedia untuk role Anda.
      </div>
    );
  }

  // Dates only (YYYY-MM-DD) — /leads reads created_from/created_to as
  // <input type="date"> values and converts them to UTC bounds itself
  // (leads-list.tsx's dateInputToStartOfDayUTC/EndOfDayUTC), not as
  // full ISO timestamps.
  function leadsLink(extra: Record<string, string> = {}): string {
    const params = new URLSearchParams({
      created_from: range.from.slice(0, 10),
      created_to: range.to.slice(0, 10),
      ...extra,
    });
    return `/leads?${params.toString()}`;
  }

  const wonCount = statusCount(summary, "won");

  return (
    <div>
      <div className="mb-4.5 flex items-center justify-between">
        <p className="text-[13px] text-muted-foreground">Ringkasan performa bisnis Anda</p>
        <select
          value={period}
          onChange={(e) => setPeriod(e.target.value as MetricsPeriod)}
          className="h-8 rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {METRICS_PERIODS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </select>
      </div>

      <FormErrorBanner message={error} />

      <div className="mb-3.5 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Lead masuk (periode ini)"
          value={loading ? "…" : String(summary?.total_new ?? 0)}
          href={leadsLink()}
        />
        <StatCard
          label="Belum ter-assign"
          value={loading ? "…" : String(summary?.unassigned ?? 0)}
          valueColor={summary && summary.unassigned > 0 ? STATUS_META.lost.color : undefined}
          href={leadsLink({ assigned_to: "none" })}
        />
        <StatCard
          label="Lead menang"
          value={loading ? "…" : String(wonCount)}
          valueColor={STATUS_META.won.color}
          href={leadsLink({ status: "won" })}
        />
        {/* Not linked to /leads — it's a computed ratio, not a single
            status filter, so there's no one list it corresponds to. */}
        <StatCard
          label="Conversion rate"
          value={loading ? "…" : formatConversionRate(summary?.conversion_rate ?? null)}
          isEmpty={!loading && (summary?.conversion_rate ?? null) === null}
        />
      </div>

      {/* freeze 3.2's "lead per status" metric — not in the mockup's 4
          cards at all (that only surfaces "Lead menang"), added as its
          own quick-link row using the same STATUS_META badges the lead
          list uses. */}
      <div className="mb-5 flex flex-wrap gap-1.5">
        {LEAD_STATUSES.map((status) => {
          const meta = STATUS_META[status];
          return (
            <Link
              key={status}
              href={leadsLink({ status })}
              className="flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
              style={{ background: meta.background, color: meta.color }}
            >
              {meta.label}
              <span className="opacity-70">{loading ? "…" : statusCount(summary, status)}</span>
            </Link>
          );
        })}
      </div>

      <Card>
        <CardContent>
          <div className="mb-3 text-[13.5px] font-semibold">Performa anggota</div>
          {!loading && employees.length === 0 ? (
            <p className="text-[13px] text-muted-foreground">
              Belum ada lead ter-assign pada periode ini.
            </p>
          ) : (
            <div className="overflow-hidden rounded-lg border border-border">
              <table className="w-full border-collapse">
                <thead>
                  <tr className="bg-muted/40">
                    <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                      Anggota
                    </th>
                    <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                      Jumlah lead
                    </th>
                    <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                      Waktu respons rata-rata
                    </th>
                    <th className="px-4 py-2.5 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                      Konversi
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {employees.map((em) => (
                    <tr key={em.membership_id} className="border-t border-border/70">
                      <td className="px-4 py-2.5 text-[13px] font-medium">{em.full_name}</td>
                      <td className="px-4 py-2.5 text-[13px]">{em.lead_count}</td>
                      <td className="px-4 py-2.5 text-[13px]">
                        {formatAvgResponseSeconds(em.avg_response_seconds)}
                      </td>
                      <td className="px-4 py-2.5 text-[13px]">{em.converted_count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
