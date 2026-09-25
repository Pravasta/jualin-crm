"use client";

// Laporan (Phase 8.6 #171, brief §10, td.md §4.4) — eight blocks, every one
// from the real /v1/metrics/* API:
//
//   1 Ringkasan · 2 Distribusi status · 3 Performa anggota   (Phase 3 API)
//   4 Tren · 5 Sumber · 6 Alasan kalah · 7 Waktu respons · 8 Tugas   (#169, #170)
//
// One filter row above everything (period, anggota, sumber), reflected in
// the URL so a report can be reloaded or shared. Each block loads and fails
// on its own: one slow or broken endpoint never blanks the page. While a
// filter change reloads, a block keeps its previous numbers dimmed rather
// than flashing to a skeleton.
//
// Out of scope and deliberately absent: money of any kind, export, custom
// report builders (brief §10.1, §15).
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { ArrowDown, ArrowUp } from "lucide-react";

import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/status-badge";
import { globalMessage } from "@/lib/auth-errors";
import {
  LEAD_SOURCES,
  LEAD_STATUSES,
  LOST_REASON_LABELS,
  SOURCE_LABELS,
  STATUS_META,
  type LeadSource,
} from "@/lib/labels";
import { listMemberships, type Member } from "@/lib/memberships";
import {
  formatAvgResponseSeconds,
  formatConversionRate,
  getMetricsEmployees,
  getMetricsLostReasons,
  getMetricsResponseTimes,
  getMetricsSources,
  getMetricsSummary,
  getMetricsTasks,
  getMetricsTrend,
  RESPONSE_BUCKET_LABELS,
  statusCount,
  type MetricsSummaryFilter,
} from "@/lib/metrics";
import { customRange, presetRange, REPORT_PRESETS, type ReportPreset, type ReportRange } from "@/lib/report-period";
import { dataVolume, memberRows, type MemberSortKey } from "@/lib/report-rows";
import { cn } from "@/lib/utils";

import { BarList, TrendColumns } from "./report-charts";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

// One block's data: loads on its own, fails on its own, and keeps the last
// good render (dimmed) while a new filter loads.
function useReport<T>(
  load: (filter: MetricsSummaryFilter, signal: AbortSignal) => Promise<T>,
  filter: MetricsSummaryFilter | null,
  key: string
) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loadedKey, setLoadedKey] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const requestKey = `${key}#${retry}`;

  useEffect(() => {
    if (!filter) return;
    const controller = new AbortController();
    load(filter, controller.signal)
      .then((d) => {
        setData(d);
        setError(null);
        setLoadedKey(requestKey);
      })
      .catch((err) => {
        if (isAbortError(err)) return;
        setError(globalMessage(err));
        setLoadedKey(requestKey);
      });
    return () => controller.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [requestKey, filter === null]);

  return { data, error, loading: loadedKey !== requestKey, retry: () => setRetry((r) => r + 1) };
}

function Block({
  title,
  note,
  loading,
  hasData,
  error,
  onRetry,
  className,
  children,
}: {
  title: string;
  /** What the numbers mean — said on screen, not left to be guessed. */
  note?: React.ReactNode;
  loading: boolean;
  hasData: boolean;
  error: string | null;
  onRetry: () => void;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <section className={cn("flex min-w-0 flex-col gap-3 rounded-xl border border-border bg-card p-4 md:p-5", className)}>
      <div>
        <h2 className="text-[15px] font-bold">{title}</h2>
        {note && <p className="mt-0.5 text-[12.5px] text-muted-foreground">{note}</p>}
      </div>
      {error ? (
        <div className="flex flex-wrap items-center gap-2.5 text-[13.5px] text-destructive">
          {error}
          <Button variant="outline" onClick={onRetry} className="h-9 bg-card text-foreground md:h-8">
            Coba lagi
          </Button>
        </div>
      ) : !hasData ? (
        <div aria-busy="true" aria-label={`Memuat ${title}`} className="flex flex-col gap-2">
          <div className="h-5 w-3/4 animate-pulse rounded bg-muted" />
          <div className="h-5 w-1/2 animate-pulse rounded bg-muted" />
          <div className="h-5 w-2/3 animate-pulse rounded bg-muted" />
        </div>
      ) : (
        <div className={cn("min-w-0 transition-opacity", loading && "opacity-50")}>{children}</div>
      )}
    </section>
  );
}

function Tile({
  label,
  value,
  href,
  muted,
  hint,
  style,
}: {
  label: string;
  value: string;
  href?: string;
  muted?: boolean;
  hint?: string;
  style?: React.CSSProperties;
}) {
  const body = (
    <>
      <div className="text-[12.5px] font-semibold text-muted-foreground">{label}</div>
      <div
        className={cn(
          "mt-1 leading-tight font-extrabold",
          muted ? "text-[15px] text-muted-foreground" : "text-[28px]"
        )}
        style={muted ? undefined : style}
      >
        {value}
      </div>
      {hint && <div className="mt-1 text-[12px] text-muted-foreground">{hint}</div>}
    </>
  );
  const cls = "block rounded-[10px] border border-border bg-card p-3.5";
  return href ? (
    <Link href={href} className={cn(cls, "hover:border-primary")}>
      {body}
    </Link>
  ) : (
    <div className={cls}>{body}</div>
  );
}

const SORT_LABELS: Record<MemberSortKey, string> = {
  lead_count: "Lead",
  avg_response_seconds: "Waktu respons",
  converted_count: "Dikonversi",
  converted_share: "% dikonversi",
};

function percent(share: number | null): string {
  return share === null ? "—" : `${Math.round(share * 100)}%`;
}

export function ReportsScreen() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const preset = (REPORT_PRESETS.find((p) => p.value === searchParams.get("period"))?.value ?? "30d") as ReportPreset;
  const customFrom = searchParams.get("from") ?? "";
  const customTo = searchParams.get("to") ?? "";
  const assignedTo = searchParams.get("assigned_to") ?? "";
  const sourceParam = searchParams.get("source") ?? "";
  const source = (LEAD_SOURCES as readonly string[]).includes(sourceParam) ? (sourceParam as LeadSource) : undefined;

  const [members, setMembers] = useState<Member[]>([]);
  const [sort, setSort] = useState<MemberSortKey>("lead_count");

  useEffect(() => {
    const controller = new AbortController();
    listMemberships(controller.signal)
      .then(setMembers)
      .catch(() => {});
    return () => controller.abort();
  }, []);

  function updateParams(patch: Record<string, string | null>) {
    const params = new URLSearchParams(searchParams.toString());
    for (const [k, v] of Object.entries(patch)) {
      if (v === null || v === "") params.delete(k);
      else params.set(k, v);
    }
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  }

  // The range is recomputed from the URL, with "now" read once per URL
  // change — a report left open overnight doesn't silently drift.
  const period = useMemo((): { range: ReportRange } | { problem: string } => {
    if (preset === "custom") return customRange(customFrom, customTo);
    return { range: presetRange(preset, new Date()) };
  }, [preset, customFrom, customTo]);
  const range = "range" in period ? period.range : null;

  const filter: MetricsSummaryFilter | null = range
    ? { createdFrom: range.from, createdTo: range.to, assignedTo: assignedTo || undefined, source }
    : null;
  const key = JSON.stringify(filter);

  const summary = useReport(getMetricsSummary, filter, key);
  const employees = useReport(getMetricsEmployees, filter, key);
  const trend = useReport(getMetricsTrend, filter, key);
  const sources = useReport(getMetricsSources, filter, key);
  const lostReasons = useReport(getMetricsLostReasons, filter, key);
  const responseTimes = useReport(getMetricsResponseTimes, filter, key);
  const tasks = useReport(getMetricsTasks, filter, key);

  // Links into the lead list carry the same period and filters, so the
  // number clicked is the number that list counts.
  function leadsLink(extra: Record<string, string> = {}): string {
    const params = new URLSearchParams(
      range ? { created_from: range.fromDate, created_to: range.toDate } : {}
    );
    if (assignedTo) params.set("assigned_to", assignedTo);
    if (source) params.set("source", source);
    for (const [k, v] of Object.entries(extra)) params.set(k, v);
    return `/leads?${params.toString()}`;
  }

  const hasNarrowing = Boolean(assignedTo || source);
  const volume = summary.data ? dataVolume(summary.data.total_new) : null;
  const rows = useMemo(() => memberRows(employees.data ?? [], sort), [employees.data, sort]);

  const selectClass =
    "h-11 w-full min-w-0 rounded-lg border border-input bg-card px-3 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-9 md:w-auto md:text-[13.5px]";

  return (
    <div className="mx-auto flex w-full max-w-[1280px] flex-col gap-4">
      {/* One filter row, above everything it scopes (dataviz: filters sit
          above the charts, never inside a card). */}
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-3 md:flex md:flex-wrap md:items-center">
        <select
          value={preset}
          onChange={(e) => updateParams({ period: e.target.value, from: null, to: null })}
          aria-label="Periode"
          className={selectClass}
        >
          {REPORT_PRESETS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </select>
        <select
          value={assignedTo}
          onChange={(e) => updateParams({ assigned_to: e.target.value || null })}
          aria-label="Anggota"
          className={selectClass}
        >
          <option value="">Semua anggota</option>
          {members.map((m) => (
            <option key={m.id} value={m.id}>
              {m.full_name}
            </option>
          ))}
        </select>
        <select
          value={source ?? ""}
          onChange={(e) => updateParams({ source: e.target.value || null })}
          aria-label="Sumber"
          className={selectClass}
        >
          <option value="">Semua sumber</option>
          {LEAD_SOURCES.map((s) => (
            <option key={s} value={s}>
              {SOURCE_LABELS[s]}
            </option>
          ))}
        </select>
        {preset === "custom" && (
          <div className="flex items-center gap-2 sm:col-span-3">
            <input
              type="date"
              value={customFrom}
              onChange={(e) => updateParams({ from: e.target.value || null })}
              aria-label="Dari tanggal"
              className="h-11 min-w-0 flex-1 rounded-lg border border-input bg-card px-2.5 text-base md:h-9 md:flex-none md:text-[13.5px]"
            />
            <span className="text-[13px] text-muted-foreground">s/d</span>
            <input
              type="date"
              value={customTo}
              onChange={(e) => updateParams({ to: e.target.value || null })}
              aria-label="Sampai tanggal"
              className="h-11 min-w-0 flex-1 rounded-lg border border-input bg-card px-2.5 text-base md:h-9 md:flex-none md:text-[13.5px]"
            />
          </div>
        )}
      </div>

      {"problem" in period && (
        <p role="alert" className="rounded-lg border border-border bg-card px-3.5 py-3 text-[14px] text-muted-foreground">
          {period.problem}
        </p>
      )}

      {volume === "empty" && (
        <div className="rounded-xl border border-dashed border-border bg-card px-5 py-12 text-center">
          <div className="mb-1.5 text-[16px] font-bold">
            {hasNarrowing ? "Tidak ada lead yang cocok dengan filter ini" : "Belum ada lead pada periode ini"}
          </div>
          <p className="mx-auto mb-4.5 max-w-[46ch] text-[14px] text-muted-foreground">
            {hasNarrowing
              ? "Coba pilih semua anggota atau semua sumber, atau lebarkan periodenya."
              : "Laporan terisi begitu lead masuk — dicatat manual, lewat formulir di situs Anda, API, atau webhook."}
          </p>
          {!hasNarrowing && (
            <div className="flex flex-wrap justify-center gap-2">
              <Button variant="outline" onClick={() => router.push("/leads")} className="bg-card md:h-9">
                Buka Lead
              </Button>
              <Button onClick={() => router.push("/connect")} className="md:h-9 md:px-4">
                Hubungkan sumber lead
              </Button>
            </div>
          )}
        </div>
      )}

      {volume === "few" && summary.data && (
        <p className="rounded-lg border border-border bg-secondary px-3.5 py-2.5 text-[13.5px] text-secondary-foreground">
          Data masih sedikit ({summary.data.total_new} lead). Angka di bawah tetap ditampilkan, tapi belum bisa
          dijadikan pegangan — satu lead saja bisa menggeser persentasenya jauh.
        </p>
      )}

      {volume !== "empty" && filter && (
        <>
          {/* 1 — Ringkasan */}
          <Block
            title="Ringkasan"
            loading={summary.loading}
            hasData={summary.data !== null}
            error={summary.error}
            onRetry={summary.retry}
          >
            {summary.data && (
              <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
                <Tile label="Lead masuk" value={String(summary.data.total_new)} href={leadsLink()} />
                <Tile
                  label="Conversion rate"
                  value={formatConversionRate(summary.data.conversion_rate)}
                  muted={summary.data.conversion_rate === null}
                  hint="Menang ÷ lead, tanpa Spam & Tidak Memenuhi Syarat"
                />
                <Tile
                  label="Belum ter-assign"
                  value={String(summary.data.unassigned)}
                  href={leadsLink({ assigned_to: "none" })}
                  style={summary.data.unassigned > 0 ? { color: "var(--accent-strong)" } : undefined}
                />
                <Tile
                  label="Lead Menang"
                  value={String(statusCount(summary.data, "won"))}
                  href={leadsLink({ status: "won" })}
                  style={{ color: STATUS_META.won.color }}
                />
              </div>
            )}
          </Block>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {/* 2 — Distribusi status. Not a funnel: it is where the period's
                leads stand NOW, not how many ever passed each stage. */}
            <Block
              title="Distribusi status"
              note="Status saat ini dari lead yang masuk pada periode ini — bukan jumlah yang pernah melewati tiap tahap."
              loading={summary.loading}
              hasData={summary.data !== null}
              error={summary.error}
              onRetry={summary.retry}
            >
              {summary.data && (
                <BarList
                  ariaLabel="Jumlah lead per status"
                  rows={LEAD_STATUSES.map((s) => ({
                    key: s,
                    // The badge (text + shape) is the identity; color only
                    // repeats it. The 8 status hues fail as a color-only
                    // categorical palette (Spam vs Tidak Memenuhi Syarat,
                    // ΔE 3.0 — notes.md "## #171").
                    label: <StatusBadge status={s} />,
                    value: Number(statusCount(summary.data, s)),
                    color: STATUS_META[s].color,
                    href: leadsLink({ status: s }),
                  }))}
                />
              )}
            </Block>

            {/* 4 — Tren */}
            <Block
              title="Tren lead masuk"
              note={trend.data?.bucket === "week" ? "Per minggu (mulai Senin)." : "Per hari."}
              loading={trend.loading}
              hasData={trend.data !== null}
              error={trend.error}
              onRetry={trend.retry}
            >
              {trend.data && <TrendColumns points={trend.data.points} bucket={trend.data.bucket} />}
            </Block>
          </div>

          {/* 3 — Performa anggota */}
          <Block
            title="Performa anggota"
            note="% dikonversi = lead yang jadi Customer ÷ lead yang dipegang anggota itu. Bukan conversion rate organisasi di atas, yang tidak menghitung Spam dan Tidak Memenuhi Syarat."
            loading={employees.loading}
            hasData={employees.data !== null}
            error={employees.error}
            onRetry={employees.retry}
          >
            {rows.length === 0 ? (
              <p className="text-[13.5px] text-muted-foreground">Belum ada anggota dengan lead pada periode ini.</p>
            ) : (
              <>
                <label className="mb-3 flex items-center gap-2 text-[13px] text-muted-foreground md:hidden">
                  Urutkan
                  <select
                    value={sort}
                    onChange={(e) => setSort(e.target.value as MemberSortKey)}
                    className="h-11 flex-1 rounded-lg border border-input bg-card px-3 text-base text-foreground"
                  >
                    {Object.entries(SORT_LABELS).map(([k, label]) => (
                      <option key={k} value={k}>
                        {label}
                      </option>
                    ))}
                  </select>
                </label>
                <table className="hidden w-full table-fixed border-collapse text-[14px] md:table">
                  <thead>
                    <tr className="border-b border-border text-left text-[11.5px] font-bold tracking-[0.05em] text-muted-foreground uppercase">
                      <th className="py-2 pr-3">Anggota</th>
                      {(Object.keys(SORT_LABELS) as MemberSortKey[]).map((k) => (
                        <th key={k} className="w-36 py-2 pr-3" aria-sort={sort === k ? (k === "avg_response_seconds" ? "ascending" : "descending") : undefined}>
                          <button
                            type="button"
                            onClick={() => setSort(k)}
                            className={cn(
                              "inline-flex items-center gap-1 uppercase",
                              sort === k ? "text-foreground" : "hover:text-foreground"
                            )}
                          >
                            {SORT_LABELS[k]}
                            {sort === k &&
                              (k === "avg_response_seconds" ? (
                                <ArrowUp className="size-3" aria-hidden />
                              ) : (
                                <ArrowDown className="size-3" aria-hidden />
                              ))}
                          </button>
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((r) => (
                      <tr key={r.membership_id} className="border-b border-border/50 last:border-b-0">
                        <td className="truncate py-2.5 pr-3">
                          <Link href={leadsLink({ assigned_to: r.membership_id })} className="font-semibold hover:underline">
                            {r.full_name}
                          </Link>
                        </td>
                        <td className="py-2.5 pr-3 tabular-nums">{r.lead_count}</td>
                        <td className="py-2.5 pr-3 text-muted-foreground">{formatAvgResponseSeconds(r.avg_response_seconds)}</td>
                        <td className="py-2.5 pr-3 tabular-nums">{r.converted_count}</td>
                        <td className="py-2.5 pr-3 tabular-nums">{percent(r.converted_share)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <ul className="flex flex-col gap-2 md:hidden">
                  {rows.map((r) => (
                    <li key={r.membership_id}>
                      <Link
                        href={leadsLink({ assigned_to: r.membership_id })}
                        className="block rounded-lg border border-border px-3 py-2.5"
                      >
                        <div className="text-[14.5px] font-bold">{r.full_name}</div>
                        <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[13px] text-muted-foreground">
                          <span>{r.lead_count} lead</span>
                          <span>Respons {formatAvgResponseSeconds(r.avg_response_seconds)}</span>
                          <span>
                            {r.converted_count} dikonversi ({percent(r.converted_share)})
                          </span>
                        </div>
                      </Link>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </Block>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
            {/* 5 — Sumber */}
            <Block
              title="Sumber lead"
              note="Conversion rate per sumber memakai definisi yang sama dengan Ringkasan."
              loading={sources.loading}
              hasData={sources.data !== null}
              error={sources.error}
              onRetry={sources.retry}
            >
              {sources.data && (
                <>
                  <BarList
                    ariaLabel="Jumlah lead per sumber"
                    rows={sources.data.map((s) => ({
                      key: s.source,
                      label: SOURCE_LABELS[s.source],
                      value: s.count,
                      href: leadsLink({ source: s.source }),
                    }))}
                  />
                  <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1 border-t border-border pt-3 text-[13px]">
                    {sources.data.map((s) => (
                      <div key={s.source} className="flex justify-between gap-2">
                        <dt className="text-muted-foreground">{SOURCE_LABELS[s.source]}</dt>
                        <dd className="font-semibold tabular-nums">
                          {s.conversion_rate === null ? "—" : `${Math.round(s.conversion_rate * 100)}%`}
                        </dd>
                      </div>
                    ))}
                  </dl>
                </>
              )}
            </Block>

            {/* 6 — Alasan kalah — one hue: every row is a Kalah lead. */}
            <Block
              title="Alasan kalah"
              loading={lostReasons.loading}
              hasData={lostReasons.data !== null}
              error={lostReasons.error}
              onRetry={lostReasons.retry}
            >
              {lostReasons.data &&
                (lostReasons.data.every((r) => r.count === 0) ? (
                  <p className="text-[13.5px] text-muted-foreground">Tidak ada lead Kalah pada periode ini.</p>
                ) : (
                  <BarList
                    ariaLabel="Jumlah lead Kalah per alasan"
                    rows={lostReasons.data.map((r) => ({
                      key: r.reason,
                      label: LOST_REASON_LABELS[r.reason],
                      value: r.count,
                      color: STATUS_META.lost.color,
                    }))}
                  />
                ))}
            </Block>

            {/* 7 — Waktu respons */}
            <Block
              title="Waktu respons"
              note="Dari lead masuk sampai aktivitas pertama anggota."
              loading={responseTimes.loading}
              hasData={responseTimes.data !== null}
              error={responseTimes.error}
              onRetry={responseTimes.retry}
            >
              {responseTimes.data && (
                <>
                  <div className="mb-3 text-[13.5px]">
                    <span className="text-muted-foreground">Median </span>
                    <span className="font-bold">{formatAvgResponseSeconds(responseTimes.data.median_seconds)}</span>
                  </div>
                  <BarList
                    ariaLabel="Jumlah lead per waktu respons"
                    rows={responseTimes.data.buckets.map((b) => ({
                      key: b.bucket,
                      label: RESPONSE_BUCKET_LABELS[b.bucket],
                      value: b.count,
                      // Untouched isn't the slowest response — it's no
                      // response. Neutral, and its label says so.
                      color: b.bucket === "never_touched" ? "var(--muted-foreground)" : undefined,
                    }))}
                  />
                </>
              )}
            </Block>
          </div>

          {/* 8 — Tugas */}
          <Block
            title="Tugas per anggota"
            note={
              <>
                <b>Selesai</b> mengikuti periode. <b>Terlambat</b> adalah keadaan <b>sekarang</b>, bukan pada periode
                itu — riwayat keterlambatan tidak disimpan. Tugas tanpa penanggung jawab tidak dihitung di sini;{" "}
                <Link href="/tasks?status=open" className="font-semibold text-accent-strong underline underline-offset-2">
                  lihat semua tugas
                </Link>
                .
              </>
            }
            loading={tasks.loading}
            hasData={tasks.data !== null}
            error={tasks.error}
            onRetry={tasks.retry}
          >
            {tasks.data && (
              <table className="w-full table-fixed border-collapse text-[14px]">
                <thead>
                  <tr className="border-b border-border text-left text-[11.5px] font-bold tracking-[0.05em] text-muted-foreground uppercase">
                    <th className="py-2 pr-3">Anggota</th>
                    {/* Narrow number columns on a phone, headers allowed to wrap:
                        the first try left 90px for a name and cut
                        "Andi Pratama" (#171). */}
                    <th className="w-[4.5rem] py-2 pr-3 text-right align-bottom md:w-32">Selesai</th>
                    <th className="w-[5.5rem] py-2 text-right align-bottom md:w-40">Terlambat sekarang</th>
                  </tr>
                </thead>
                <tbody>
                  {tasks.data.map((t) => (
                    <tr key={t.membership_id} className="border-b border-border/50 last:border-b-0">
                      <td className="truncate py-2.5 pr-3 font-semibold">{t.full_name}</td>
                      <td className="py-2.5 pr-3 text-right tabular-nums">{t.completed_count}</td>
                      <td
                        className={cn(
                          "py-2.5 text-right tabular-nums",
                          t.overdue_count > 0 && "font-bold text-destructive"
                        )}
                      >
                        {t.overdue_count}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Block>
        </>
      )}
    </div>
  );
}
