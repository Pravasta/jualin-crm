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
import { cn } from "@/lib/utils";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

// One KPI tile (Phase 8.6, #163). A link whenever the number corresponds to
// exactly one lead list — brief §8.3: every number opens its filtered list.
function StatCard({
  label,
  value,
  loading,
  isEmpty,
  hint,
  valueClassName,
  valueStyle,
  href,
}: {
  label: string;
  value: string;
  loading: boolean;
  /** "Belum ada data" — styled apart from a real 0 (brief §8.3). */
  isEmpty?: boolean;
  hint?: string;
  valueClassName?: string;
  valueStyle?: React.CSSProperties;
  href?: string;
}) {
  const content = (
    <>
      <div className="text-[12.5px] font-semibold text-muted-foreground">{label}</div>
      {loading ? (
        <div aria-hidden className="mt-2 h-8 w-16 animate-pulse rounded-md bg-muted" />
      ) : isEmpty ? (
        <div className="mt-1.5 text-[15px] font-semibold text-muted-foreground">{value}</div>
      ) : (
        <div
          className={cn("mt-1 text-[28px] leading-tight font-extrabold tabular-nums", valueClassName)}
          style={valueStyle}
        >
          {value}
        </div>
      )}
      {hint && <div className="mt-1 text-[12px] text-muted-foreground">{hint}</div>}
    </>
  );
  const className = "block rounded-[10px] border border-border bg-card p-4 text-left";
  if (href) {
    return (
      <Link href={href} className={cn(className, "transition-colors hover:border-primary")}>
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
      <div className="rounded-xl border border-dashed border-border bg-card px-6 py-12 text-center text-sm text-muted-foreground">
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
  const conversionMissing = !loading && (summary?.conversion_rate ?? null) === null;

  return (
    <div className="flex w-full flex-col gap-3.5 md:gap-4.5">
      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <p className="text-[14px] text-muted-foreground">Bagaimana keadaan bisnis periode ini?</p>
        <select
          value={period}
          onChange={(e) => setPeriod(e.target.value as MetricsPeriod)}
          aria-label="Periode"
          className="h-11 rounded-lg border border-input bg-card px-3 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-9 md:text-[14px]"
        >
          {METRICS_PERIODS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </select>
      </div>

      <FormErrorBanner message={error} />

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard
          label="Lead masuk"
          loading={loading}
          value={String(summary?.total_new ?? 0)}
          href={leadsLink()}
        />
        <StatCard
          label="Belum ter-assign"
          loading={loading}
          value={String(summary?.unassigned ?? 0)}
          valueClassName={summary && summary.unassigned > 0 ? "text-accent-strong" : undefined}
          href={leadsLink({ assigned_to: "none" })}
        />
        {/* Not linked to /leads — it's a computed ratio, not a single
            status filter, so there's no one list it corresponds to. */}
        <StatCard
          label="Conversion rate"
          loading={loading}
          value={formatConversionRate(summary?.conversion_rate ?? null)}
          isEmpty={conversionMissing}
          hint="Tanpa Spam & Tidak Memenuhi Syarat"
        />
        <StatCard
          label="Lead Menang"
          loading={loading}
          value={String(wonCount)}
          valueStyle={{ color: STATUS_META.won.color }}
          href={leadsLink({ status: "won" })}
        />
      </div>

      {/* freeze 3.2's "lead per status" — one chip per status, each opening
          its filtered list. Outlined in the status color, the same pair the
          badges use on white (5.03–5.27:1, #159). */}
      <Card>
        <CardContent className="flex flex-col gap-3">
          <h2 className="text-[15px] font-bold">Jumlah per status</h2>
          <div className="flex flex-wrap gap-2">
            {LEAD_STATUSES.map((status) => {
              const meta = STATUS_META[status];
              return (
                <Link
                  key={status}
                  href={leadsLink({ status })}
                  className="inline-flex min-h-9 items-center rounded-full border-[1.5px] bg-card px-3 text-[12.5px] font-semibold whitespace-nowrap md:min-h-8"
                  style={{ borderColor: meta.color, color: meta.color }}
                >
                  {meta.label} · {loading ? "…" : statusCount(summary, status)}
                </Link>
              );
            })}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 className="text-[15px] font-bold">Performa per anggota</h2>
            {/* The shortcut #163 deferred until the route existed (#171). */}
            <Link href="/reports" className="text-[13.5px] font-semibold text-accent-strong hover:underline">
              Lihat Laporan lengkap →
            </Link>
          </div>
          {loading ? (
            <div aria-busy="true" aria-label="Memuat performa anggota" className="flex flex-col gap-2">
              {Array.from({ length: 3 }, (_, i) => (
                <div key={i} className="h-11 animate-pulse rounded-md bg-muted" />
              ))}
            </div>
          ) : employees.length === 0 ? (
            <p className="text-[13.5px] text-muted-foreground">Belum ada lead ter-assign pada periode ini.</p>
          ) : (
            <>
              {/* 768px and up: a table. Every row opens that member's leads
                  for the same period (brief §8.3: every number is a link). */}
              <table className="hidden w-full table-fixed border-collapse text-[14px] md:table">
                <thead>
                  <tr className="border-b border-border text-left text-[11.5px] font-bold tracking-[0.05em] text-muted-foreground uppercase">
                    <th className="py-2 pr-3">Anggota</th>
                    <th className="w-28 py-2 pr-3">Lead</th>
                    <th className="w-52 py-2 pr-3">Waktu respons rata-rata</th>
                    <th className="w-28 py-2">Konversi</th>
                  </tr>
                </thead>
                <tbody>
                  {employees.map((em) => (
                    <tr key={em.membership_id} className="border-b border-border/50 last:border-b-0">
                      <td className="truncate py-2.5 pr-3">
                        <Link href={leadsLink({ assigned_to: em.membership_id })} className="font-semibold hover:underline">
                          {em.full_name}
                        </Link>
                      </td>
                      <td className="py-2.5 pr-3 tabular-nums">{em.lead_count}</td>
                      <td className="py-2.5 pr-3 text-muted-foreground">
                        {formatAvgResponseSeconds(em.avg_response_seconds)}
                      </td>
                      <td className="py-2.5 tabular-nums">{em.converted_count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>

              {/* Below 768px: one card per member, same link. */}
              <ul className="flex flex-col gap-2 md:hidden">
                {employees.map((em) => (
                  <li key={em.membership_id}>
                    <Link
                      href={leadsLink({ assigned_to: em.membership_id })}
                      className="block rounded-lg border border-border px-3 py-2.5 active:bg-muted/60"
                    >
                      <div className="text-[14.5px] font-bold">{em.full_name}</div>
                      <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[13px] text-muted-foreground">
                        <span>{em.lead_count} lead</span>
                        <span>Respons {formatAvgResponseSeconds(em.avg_response_seconds)}</span>
                        <span>{em.converted_count} konversi</span>
                      </div>
                    </Link>
                  </li>
                ))}
              </ul>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
