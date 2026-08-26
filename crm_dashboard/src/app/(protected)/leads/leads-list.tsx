"use client";

// Highest-traffic screen in the product (freeze 3.2) — filters combine,
// the "tanpa pemilik aktif" chip is permanent (freeze 2.3 ketentuan #3),
// and the URL is the source of truth for filter state so the page can be
// reloaded or shared without losing context (issue #32 acceptance
// criteria).
import { useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FormErrorBanner } from "@/components/form-error-banner";
import { dateInputToEndOfDayUTC, dateInputToStartOfDayUTC, formatDateID } from "@/lib/date";
import { hasAnyLeadFilter, parseCSVParam, toggleCSVValue } from "@/lib/lead-filters";
import { listLeads, type Lead } from "@/lib/leads";
import { listMemberships, type Member } from "@/lib/memberships";
import { getMetricsSummary, statusCount, type MetricsSummary } from "@/lib/metrics";
import { LEAD_SOURCES, LEAD_STATUSES, SOURCE_LABELS, STATUS_META } from "@/lib/labels";
import { useDebouncedValue } from "@/lib/use-debounced-value";
import { globalMessage } from "@/lib/auth-errors";
import { NewLeadDialog } from "./new-lead-dialog";

const PER_PAGE = 25;

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function LeadsList() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const statusFilter = useMemo(() => parseCSVParam(searchParams.get("status")), [searchParams]);
  const sourceFilter = useMemo(() => parseCSVParam(searchParams.get("source")), [searchParams]);
  const assignedTo = searchParams.get("assigned_to") ?? "";
  const createdFromInput = searchParams.get("created_from") ?? "";
  const createdToInput = searchParams.get("created_to") ?? "";
  const page = Math.max(1, Number(searchParams.get("page")) || 1);
  const urlKeyword = searchParams.get("q") ?? "";

  // Local — debounced OUT to the URL below. Keeping this separate from
  // urlKeyword avoids a two-way sync loop; the only place that resets it
  // FROM the URL is handleClearFilters.
  const [keywordInput, setKeywordInput] = useState(urlKeyword);
  const debouncedKeyword = useDebouncedValue(keywordInput, 300);

  const [leads, setLeads] = useState<Lead[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  // "loading" is derived, not a setState the fetch effect calls directly —
  // react-hooks/set-state-in-effect forbids synchronous setState at the
  // top of an effect body (only inside .then/.catch is fine, since that
  // runs after a real async gap). Comparing "what we last successfully
  // rendered" against "what the current filters ask for" gives the same
  // signal without that call.
  const [loadedKey, setLoadedKey] = useState<string | null>(null);

  const [members, setMembers] = useState<Member[]>([]);
  const [summary, setSummary] = useState<MetricsSummary | null>(null);

  const [newLeadOpen, setNewLeadOpen] = useState(false);
  // Bumped after a successful create to force the list effect to re-run
  // even when the URL (and therefore searchParams) doesn't change — a
  // new lead may or may not match the current filters, and re-running
  // the real query is what keeps the table and meta.total honest,
  // rather than optimistically inserting a row that might not belong.
  const [refreshKey, setRefreshKey] = useState(0);

  function updateParams(patch: Record<string, string | null>) {
    const params = new URLSearchParams(searchParams.toString());
    for (const [key, value] of Object.entries(patch)) {
      if (value === null || value === "") params.delete(key);
      else params.set(key, value);
    }
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  }

  // Any filter change resets to page 1 — staying on page 4 of a filter
  // that now has 2 results shows an empty page that looks like a bug.
  function updateFilterParams(patch: Record<string, string | null>) {
    updateParams({ ...patch, page: null });
  }

  useEffect(() => {
    if (debouncedKeyword !== urlKeyword) {
      updateFilterParams({ q: debouncedKeyword || null });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedKeyword]);

  const hasAnyFilter = hasAnyLeadFilter({
    status: statusFilter,
    source: sourceFilter,
    assignedTo,
    keyword: urlKeyword,
    createdFrom: createdFromInput,
    createdTo: createdToInput,
  });

  function handleClearFilters() {
    setKeywordInput("");
    router.replace(pathname, { scroll: false });
  }

  // Memberships: fetched once, independent of every filter — used for
  // the owner dropdown and to resolve assigned_to_membership_id to a name.
  useEffect(() => {
    const controller = new AbortController();
    listMemberships(controller.signal)
      .then(setMembers)
      .catch((err) => {
        if (!isAbortError(err)) {
          // Non-fatal for this screen: the table still works with
          // membership_id shown as "—" for every row rather than a name.
        }
      });
    return () => controller.abort();
  }, []);

  // Status/unassigned chip counts come from the aggregate endpoint, not
  // from the current page — org-wide for the selected period, not
  // narrowed by source/owner/keyword (see @/lib/metrics's doc comment).
  useEffect(() => {
    const controller = new AbortController();
    getMetricsSummary(
      {
        createdFrom: dateInputToStartOfDayUTC(createdFromInput),
        createdTo: dateInputToEndOfDayUTC(createdToInput),
      },
      controller.signal
    )
      .then(setSummary)
      .catch((err) => {
        if (!isAbortError(err)) setSummary(null);
      });
    return () => controller.abort();
  }, [createdFromInput, createdToInput]);

  const requestKey = JSON.stringify([
    statusFilter,
    sourceFilter,
    assignedTo,
    urlKeyword,
    createdFromInput,
    createdToInput,
    page,
    refreshKey,
  ]);
  const loading = loadedKey !== requestKey;

  // The list itself. Keyed on every filter + page — an AbortController
  // cancels the in-flight request when a newer one starts, so a slow
  // response for a filter the user already changed away from can't
  // clobber a faster, newer one. Every setState here runs inside
  // .then/.catch, after a real async gap — never synchronously in the
  // effect body (react-hooks/set-state-in-effect).
  useEffect(() => {
    const controller = new AbortController();
    listLeads(
      {
        status: statusFilter.length ? (statusFilter as Lead["status"][]) : undefined,
        source: sourceFilter.length ? (sourceFilter as Lead["source"][]) : undefined,
        assignedTo: assignedTo || undefined,
        q: urlKeyword || undefined,
        createdFrom: dateInputToStartOfDayUTC(createdFromInput),
        createdTo: dateInputToEndOfDayUTC(createdToInput),
        page,
        perPage: PER_PAGE,
      },
      controller.signal
    )
      .then(({ data, meta }) => {
        setLeads(data);
        setTotal(meta.total);
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
  }, [statusFilter.join(","), sourceFilter.join(","), assignedTo, urlKeyword, createdFromInput, createdToInput, page, refreshKey]);

  const membersById = useMemo(() => new Map(members.map((m) => [m.id, m])), [members]);

  const isEmptyNoData = !loading && total === 0 && !hasAnyFilter;
  const isEmptyFiltered = !loading && total === 0 && hasAnyFilter;
  const showTable = !loading && total > 0;

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const rangeStart = total === 0 ? 0 : (page - 1) * PER_PAGE + 1;
  const rangeEnd = Math.min(page * PER_PAGE, total);

  return (
    <div>
      <div className="mb-3.5 flex items-center justify-between gap-3">
        <Input
          value={keywordInput}
          onChange={(e) => setKeywordInput(e.target.value)}
          placeholder="Cari nama, email, atau telepon…"
          className="h-8.5 max-w-80"
        />
        <div className="flex items-center gap-2">
          <input
            type="date"
            value={createdFromInput}
            onChange={(e) => updateFilterParams({ created_from: e.target.value || null })}
            aria-label="Tanggal masuk dari"
            className="h-8.5 rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          />
          <span className="text-xs text-muted-foreground">s/d</span>
          <input
            type="date"
            value={createdToInput}
            onChange={(e) => updateFilterParams({ created_to: e.target.value || null })}
            aria-label="Tanggal masuk sampai"
            className="h-8.5 rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          />
          <select
            value={assignedTo}
            onChange={(e) => updateFilterParams({ assigned_to: e.target.value || null })}
            className="h-8.5 rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <option value="">Semua pemilik</option>
            {members.map((m) => (
              <option key={m.id} value={m.id}>
                {m.full_name}
              </option>
            ))}
          </select>
          <Button onClick={() => setNewLeadOpen(true)}>+ Lead baru</Button>
        </div>
      </div>

      <div className="mb-2 flex flex-wrap items-center gap-1.5">
        <button
          type="button"
          onClick={() =>
            updateFilterParams({ assigned_to: assignedTo === "none" ? null : "none" })
          }
          className="flex items-center gap-1.5 rounded-full border-[1.5px] px-2.5 py-1 text-xs font-semibold transition-colors"
          style={{
            borderColor:
              assignedTo === "none"
                ? "oklch(0.55 0.2 25)"
                : summary && summary.unassigned > 0
                  ? "oklch(0.55 0.2 25 / 45%)"
                  : "oklch(0.922 0 0)",
            background: assignedTo === "none" ? "oklch(0.55 0.2 25 / 10%)" : "#fff",
            color:
              assignedTo === "none"
                ? "oklch(0.5 0.2 25)"
                : summary && summary.unassigned > 0
                  ? "oklch(0.5 0.2 25)"
                  : "oklch(0.35 0 0)",
          }}
        >
          Tanpa pemilik aktif
          <span className="rounded-full bg-black/8 px-1.5">{summary?.unassigned ?? "…"}</span>
        </button>
        <span className="mx-1 h-4 w-px bg-border" aria-hidden />
        {LEAD_STATUSES.map((status) => {
          const active = statusFilter.includes(status);
          const meta = STATUS_META[status];
          return (
            <button
              key={status}
              type="button"
              onClick={() => updateFilterParams({ status: toggleCSVValue(statusFilter, status).join(",") || null })}
              className="flex items-center gap-1.5 rounded-full border-[1.5px] px-2.5 py-1 text-xs font-medium transition-colors"
              style={{
                borderColor: active ? meta.color : "oklch(0.922 0 0)",
                background: active ? `color-mix(in oklch, ${meta.color}, white 82%)` : "#fff",
                color: active ? meta.color : "oklch(0.35 0 0)",
              }}
            >
              {meta.label}
              <span className="opacity-60">{statusCount(summary, status)}</span>
            </button>
          );
        })}
      </div>
      <div className="mb-3.5 flex flex-wrap items-center gap-1.5">
        {LEAD_SOURCES.map((source) => {
          const active = sourceFilter.includes(source);
          return (
            <button
              key={source}
              type="button"
              onClick={() => updateFilterParams({ source: toggleCSVValue(sourceFilter, source).join(",") || null })}
              className="rounded-full border-[1.5px] px-2.5 py-1 text-xs font-medium transition-colors"
              style={{
                borderColor: active ? "oklch(0.56 0.19 41)" : "oklch(0.922 0 0)",
                background: active ? "oklch(0.56 0.19 41 / 10%)" : "#fff",
                color: active ? "oklch(0.48 0.17 41)" : "oklch(0.35 0 0)",
              }}
            >
              {SOURCE_LABELS[source]}
            </button>
          );
        })}
        {hasAnyFilter && (
          <button
            type="button"
            onClick={handleClearFilters}
            className="rounded-full px-2.5 py-1 text-xs font-medium text-accent-strong underline"
          >
            Hapus semua filter
          </button>
        )}
      </div>

      {error && <FormErrorBanner message={error} />}

      {isEmptyNoData && (
        <div className="rounded-lg border border-dashed border-muted-foreground/30 bg-background px-6 py-12 text-center">
          <div className="mb-1.5 text-[14.5px] font-semibold">Belum ada lead</div>
          <div className="mb-4 text-[13px] text-muted-foreground">
            Buat lead pertama Anda untuk mulai melacak calon pelanggan.
          </div>
          <Button onClick={() => setNewLeadOpen(true)}>+ Buat lead pertama</Button>
        </div>
      )}

      {isEmptyFiltered && (
        <div className="rounded-lg border border-dashed border-muted-foreground/30 bg-background px-6 py-12 text-center">
          <div className="mb-1.5 text-[14.5px] font-semibold">Tidak ada lead yang cocok</div>
          <div className="mb-4 text-[13px] text-muted-foreground">
            Tidak ada lead yang sesuai dengan filter saat ini.
          </div>
          <Button variant="outline" onClick={handleClearFilters}>
            Hapus filter
          </Button>
        </div>
      )}

      {loading && leads.length === 0 && !error && (
        <div className="rounded-lg border border-border bg-background px-6 py-12 text-center text-sm text-muted-foreground">
          Memuat…
        </div>
      )}

      {showTable && (
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <div className="overflow-x-auto">
            <table className="w-full border-collapse">
              <thead>
                <tr className="bg-muted/40">
                  <th className="px-3.5 py-2 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Lead
                  </th>
                  <th className="px-3.5 py-2 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Status
                  </th>
                  <th className="px-3.5 py-2 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Pemilik
                  </th>
                  <th className="px-3.5 py-2 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Sumber
                  </th>
                  <th className="px-3.5 py-2 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Tanggal masuk
                  </th>
                </tr>
              </thead>
              <tbody>
                {leads.map((lead) => {
                  const owner = lead.assigned_to_membership_id
                    ? membersById.get(lead.assigned_to_membership_id)?.full_name
                    : null;
                  const statusMeta = STATUS_META[lead.status];
                  return (
                    <tr
                      key={lead.id}
                      onClick={() => router.push(`/leads/${lead.id}`)}
                      className="cursor-pointer border-t border-border/70 hover:bg-muted/40"
                    >
                      <td className="px-3.5 py-2.5">
                        <div className="text-[13px] font-medium">{lead.name}</div>
                        <div className="text-[11.5px] text-muted-foreground">
                          #{lead.lead_number}
                          {lead.email ? ` · ${lead.email}` : ""}
                        </div>
                      </td>
                      <td className="px-3.5 py-2.5">
                        <span
                          className="inline-block rounded-full px-2.5 py-0.5 text-xs font-medium"
                          style={{ background: statusMeta.background, color: statusMeta.color }}
                        >
                          {statusMeta.label}
                        </span>
                      </td>
                      <td
                        className="px-3.5 py-2.5 text-[13px]"
                        style={owner ? undefined : { color: "oklch(0.65 0 0)", fontStyle: "italic" }}
                      >
                        {owner ?? "—"}
                      </td>
                      <td className="px-3.5 py-2.5 text-[13px] text-foreground/70">
                        {SOURCE_LABELS[lead.source]}
                      </td>
                      <td className="px-3.5 py-2.5 text-[13px] text-foreground/70">
                        {formatDateID(lead.created_at)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <div className="flex items-center justify-between border-t border-border px-3.5 py-2.5 text-[12.5px] text-muted-foreground">
            <span>
              Menampilkan {rangeStart}–{rangeEnd} dari {total} lead
            </span>
            {totalPages > 1 && (
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => updateParams({ page: String(page - 1) })}
                >
                  Sebelumnya
                </Button>
                <span>
                  Halaman {page} dari {totalPages}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= totalPages}
                  onClick={() => updateParams({ page: String(page + 1) })}
                >
                  Berikutnya
                </Button>
              </div>
            )}
          </div>
        </div>
      )}

      <NewLeadDialog
        open={newLeadOpen}
        onOpenChange={setNewLeadOpen}
        onCreated={() => {
          // A created lead always matches the current filters or not —
          // either way, re-running the real query (rather than
          // optimistically inserting a row) is what keeps the table and
          // meta.total consistent with the server's actual state. The
          // URL doesn't change here, so refreshKey is what re-triggers
          // the fetch effect.
          setRefreshKey((k) => k + 1);
        }}
      />
    </div>
  );
}
