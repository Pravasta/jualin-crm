"use client";

// The two chart forms the Laporan screen needs (Phase 8.6 #171), built from
// plain HTML — no chart library (td.md §4.4, Aturan #27). Specs follow the
// dataviz method: bars ≤24px thick with a 4px rounded data end and a square
// baseline, values at the bar tip in TEXT tokens (never the series color),
// hairline recessive axes, and every chart backed by numbers you can read
// without hovering (brief §10.4: "setiap grafik punya padanan angka").
import { useState } from "react";
import Link from "next/link";

import { cn } from "@/lib/utils";

// --- Horizontal bars -------------------------------------------------------

export interface BarRow {
  key: string;
  /** What the row is. Text (or a status badge) — identity never rests on the
   *  bar's color alone. */
  label: React.ReactNode;
  value: number;
  /** Printed at the bar tip. Defaults to the value. */
  valueText?: string;
  /** Mark color. Defaults to the primary hue (one-series magnitude). */
  color?: string;
  /** Opens the list behind the number (brief §8.3, §10). */
  href?: string;
}

// Label column on the left from 640px; stacked above the bar on a phone so a
// long label ("Tidak Memenuhi Syarat") never squeezes the bar to nothing.
export function BarList({ rows, ariaLabel }: { rows: BarRow[]; ariaLabel: string }) {
  const max = Math.max(1, ...rows.map((r) => r.value));
  return (
    <ul aria-label={ariaLabel} className="flex flex-col gap-2.5">
      {rows.map((row) => {
        const pct = (row.value / max) * 100;
        const body = (
          <div className="grid grid-cols-1 items-center gap-x-3 gap-y-1 sm:grid-cols-[10.5rem_minmax(0,1fr)]">
            <div className="min-w-0 text-[13.5px]">{row.label}</div>
            <div className="flex min-w-0 items-center gap-2">
              <div className="h-4 min-w-0 flex-1">
                {/* ≤24px thick, grows from one baseline, 4px round data end. */}
                <div
                  className="h-full rounded-r-[4px]"
                  style={{
                    width: row.value === 0 ? 0 : `max(${pct}%, 3px)`,
                    background: row.color ?? "var(--primary)",
                  }}
                />
              </div>
              <span className="w-[4.5rem] shrink-0 text-right text-[13.5px] font-semibold tabular-nums">
                {row.valueText ?? row.value}
              </span>
            </div>
          </div>
        );
        return (
          <li key={row.key}>
            {row.href ? (
              <Link href={row.href} className="-mx-1.5 block rounded-md px-1.5 py-1 hover:bg-muted/60">
                {body}
              </Link>
            ) : (
              <div className="py-1">{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}

// --- Trend columns ---------------------------------------------------------

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];

/** "2026-09-25" → "25 Sep". The API sends a calendar date in the
 *  organization's timezone; parsing it with `new Date(str)` would read it
 *  as UTC midnight and can shift the day, so it is split, not parsed. */
export function formatCalendarDate(date: string): string {
  const [, m, d] = date.split("-").map(Number);
  return `${d} ${MONTHS[m - 1]}`;
}

export interface TrendPoint {
  date: string;
  count: number;
}

// Single series → no legend (the title names it). Columns rather than a
// line: these are counts per bucket, and a count of zero must look like
// nothing happened, not like a line passing through. Hover or focus a
// column for its exact value; the table below carries every value too.
export function TrendColumns({ points, bucket }: { points: TrendPoint[]; bucket: "day" | "week" }) {
  const [active, setActive] = useState<number | null>(null);
  const max = Math.max(1, ...points.map((p) => p.count));
  const unit = bucket === "week" ? "Minggu mulai" : "";
  const describe = (p: TrendPoint) => `${unit ? `${unit} ` : ""}${formatCalendarDate(p.date)}: ${p.count} lead`;
  const ticks = [0, Math.floor((points.length - 1) / 2), points.length - 1].filter(
    (v, i, a) => a.indexOf(v) === i && v >= 0
  );

  return (
    <div className="flex flex-col gap-1.5">
      <div className="relative flex h-40 items-end gap-[2px] border-b border-border pt-5">
        {/* Top hairline with the scale's maximum — the one reference value. */}
        <div aria-hidden className="pointer-events-none absolute inset-x-0 top-5 border-t border-border/70" />
        <span aria-hidden className="absolute top-0 left-0 text-[11.5px] text-muted-foreground tabular-nums">
          {max}
        </span>

        {points.map((p, i) => (
          <button
            key={p.date}
            type="button"
            aria-label={describe(p)}
            onPointerEnter={() => setActive(i)}
            onPointerLeave={() => setActive((a) => (a === i ? null : a))}
            onFocus={() => setActive(i)}
            onBlur={() => setActive((a) => (a === i ? null : a))}
            // The hit target is the whole column slot, not the painted bar.
            className="group relative flex h-full max-w-6 min-w-0 flex-1 items-end outline-none"
          >
            <span
              className={cn(
                "block w-full rounded-t-[4px] transition-opacity",
                p.count === 0 ? "bg-transparent" : "bg-primary",
                active !== null && active !== i && "opacity-55",
                "group-focus-visible:ring-2 group-focus-visible:ring-ring"
              )}
              style={{ height: p.count === 0 ? 0 : `max(${(p.count / max) * 100}%, 3px)` }}
            />
          </button>
        ))}

        {active !== null && (
          <div
            role="status"
            className="pointer-events-none absolute top-0 z-10 -translate-x-1/2 rounded-md border border-border bg-popover px-2.5 py-1.5 text-[12.5px] whitespace-nowrap shadow-md"
            style={{ left: `clamp(3rem, ${((active + 0.5) / points.length) * 100}%, calc(100% - 3rem))` }}
          >
            {/* Value leads, label follows. */}
            <span className="font-bold tabular-nums">{points[active].count} lead</span>
            <span className="ml-1.5 text-muted-foreground">
              {unit ? `${unit} ` : ""}
              {formatCalendarDate(points[active].date)}
            </span>
          </div>
        )}
      </div>

      <div aria-hidden className="relative h-4 text-[11.5px] text-muted-foreground">
        {ticks.map((i) => (
          <span
            key={i}
            className="absolute -translate-x-1/2 whitespace-nowrap first:translate-x-0 last:-translate-x-full"
            style={{ left: `${((i + 0.5) / points.length) * 100}%` }}
          >
            {formatCalendarDate(points[i].date)}
          </span>
        ))}
      </div>

      <details className="mt-1 text-[13px]">
        <summary className="cursor-pointer font-semibold text-accent-strong">Lihat sebagai tabel</summary>
        <div className="mt-2 max-h-64 overflow-y-auto rounded-md border border-border">
          <table className="w-full text-left">
            <thead className="sticky top-0 bg-muted text-[11.5px] font-bold tracking-[0.05em] text-muted-foreground uppercase">
              <tr>
                <th className="px-3 py-1.5">{bucket === "week" ? "Minggu mulai" : "Tanggal"}</th>
                <th className="px-3 py-1.5 text-right">Lead</th>
              </tr>
            </thead>
            <tbody>
              {points.map((p) => (
                <tr key={p.date} className="border-t border-border/60">
                  <td className="px-3 py-1.5">{formatCalendarDate(p.date)}</td>
                  <td className="px-3 py-1.5 text-right tabular-nums">{p.count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
    </div>
  );
}
