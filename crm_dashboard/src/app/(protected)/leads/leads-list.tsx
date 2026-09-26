"use client";

// Highest-traffic screen in the product (freeze 3.2) — filters combine,
// the "tanpa pemilik aktif" chip is permanent (freeze 2.3 ketentuan #3),
// and the URL is the source of truth for filter state so the page can be
// reloaded or shared without losing context (issue #32 acceptance
// criteria).
//
// Phase 8.6 (#161): table from 768px up, cards below it; status chips stay
// on screen and scroll sideways on a phone, while source/owner/date move
// behind a "Filter" button into a bottom sheet. Nothing about what the
// filters mean changed — same URL params, same queries.
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Plus, Search, SlidersHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { FormErrorBanner } from "@/components/form-error-banner";
import { NoContactBadge } from "@/components/no-contact-badge";
import { StatusBadge } from "@/components/status-badge";
import { dateInputToEndOfDayUTC, dateInputToStartOfDayUTC, formatDateID } from "@/lib/date";
import { hasContact, primaryContact } from "@/lib/lead-contact";
import { hasAnyLeadFilter, parseCSVParam, sheetFilterCount, toggleCSVValue } from "@/lib/lead-filters";
import { listLeads, type Lead } from "@/lib/leads";
import { listMemberships, type Member } from "@/lib/memberships";
import { getMetricsSummary, statusCount, type MetricsSummary } from "@/lib/metrics";
import { LEAD_SOURCES, LEAD_STATUSES, SOURCE_LABELS, STATUS_META } from "@/lib/labels";
import { useDebouncedValue } from "@/lib/use-debounced-value";
import { globalMessage } from "@/lib/auth-errors";
import { cn } from "@/lib/utils";
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
  const [filterSheetOpen, setFilterSheetOpen] = useState(false);
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

  const filterState = {
    status: statusFilter,
    source: sourceFilter,
    assignedTo,
    keyword: urlKeyword,
    createdFrom: createdFromInput,
    createdTo: createdToInput,
  };
  const hasAnyFilter = hasAnyLeadFilter(filterState);
  const hiddenFilterCount = sheetFilterCount(filterState);

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

  const ownerName = (lead: Lead): string | null =>
    lead.assigned_to_membership_id
      ? (membersById.get(lead.assigned_to_membership_id)?.full_name ?? "—")
      : null;

  const isEmptyNoData = !loading && total === 0 && !hasAnyFilter;
  const isEmptyFiltered = !loading && total === 0 && hasAnyFilter;
  const showTable = !loading && total > 0;

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const rangeStart = total === 0 ? 0 : (page - 1) * PER_PAGE + 1;
  const rangeEnd = Math.min(page * PER_PAGE, total);

  // Source, owner and entry date — rendered inline from 768px up and inside
  // the Filter sheet below it. One definition, so the two can't drift.
  const secondaryFilters = (
    <>
      <div className="flex flex-col gap-1.5 md:flex-row md:items-center">
        <span className="text-[12.5px] font-semibold text-muted-foreground md:sr-only">Sumber</span>
        <div className="flex flex-wrap gap-1.5">
          {LEAD_SOURCES.map((source) => {
            const active = sourceFilter.includes(source);
            return (
              <button
                key={source}
                type="button"
                aria-pressed={active}
                onClick={() => updateFilterParams({ source: toggleCSVValue(sourceFilter, source).join(",") || null })}
                className={cn(
                  "min-h-9 rounded-full border-[1.5px] px-3 text-[13px] font-medium transition-colors md:min-h-8",
                  active
                    ? "border-primary bg-accent-tint text-accent-strong"
                    : "border-input bg-card text-foreground hover:bg-muted"
                )}
              >
                {SOURCE_LABELS[source]}
              </button>
            );
          })}
        </div>
      </div>
      <label className="flex flex-col gap-1.5 md:flex-row md:items-center">
        <span className="text-[12.5px] font-semibold text-muted-foreground md:sr-only">Pemilik</span>
        <select
          value={assignedTo}
          onChange={(e) => updateFilterParams({ assigned_to: e.target.value || null })}
          className="h-11 w-full rounded-lg border border-input bg-card px-3 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-9 md:w-auto md:text-[13.5px]"
        >
          <option value="">Semua pemilik</option>
          <option value="none">Tanpa pemilik aktif</option>
          {members.map((m) => (
            <option key={m.id} value={m.id}>
              {m.full_name}
            </option>
          ))}
        </select>
      </label>
      <div className="flex flex-col gap-1.5 md:flex-row md:items-center">
        <span className="text-[12.5px] font-semibold text-muted-foreground md:sr-only">Tanggal masuk</span>
        <div className="flex items-center gap-2">
          <input
            type="date"
            value={createdFromInput}
            onChange={(e) => updateFilterParams({ created_from: e.target.value || null })}
            aria-label="Tanggal masuk dari"
            className="h-11 min-w-0 flex-1 rounded-lg border border-input bg-card px-2.5 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-9 md:flex-none md:text-[13px]"
          />
          <span className="text-xs text-muted-foreground">s/d</span>
          <input
            type="date"
            value={createdToInput}
            onChange={(e) => updateFilterParams({ created_to: e.target.value || null })}
            aria-label="Tanggal masuk sampai"
            className="h-11 min-w-0 flex-1 rounded-lg border border-input bg-card px-2.5 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-9 md:flex-none md:text-[13px]"
          />
        </div>
      </div>
    </>
  );

  const unassignedActive = assignedTo === "none";

  return (
    // pb-16 on phones: room for the floating "Lead baru" button, so it never
    // covers the last card's status badge at the end of the scroll.
    <div className="flex w-full flex-col gap-3.5 pb-16 md:gap-4 md:pb-0">
      <div className="flex items-center gap-2.5">
        <div className="relative min-w-0 flex-1 md:max-w-96">
          <Search
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <Input
            value={keywordInput}
            onChange={(e) => setKeywordInput(e.target.value)}
            placeholder="Cari nama, email, atau telepon"
            aria-label="Cari lead"
            className="h-11 bg-card pl-9 md:h-9"
          />
        </div>
        <Button
          variant="outline"
          onClick={() => setFilterSheetOpen(true)}
          className="h-11 gap-1.5 bg-card px-3.5 text-[14px] md:hidden"
        >
          <SlidersHorizontal className="size-4" aria-hidden />
          Filter
          {hiddenFilterCount > 0 && (
            <span className="rounded-full bg-accent-strong px-1.5 text-[11px] leading-4 text-white">
              {hiddenFilterCount}
            </span>
          )}
        </Button>
        <Button onClick={() => setNewLeadOpen(true)} className="ml-auto hidden h-9 gap-1.5 px-4 md:inline-flex">
          <Plus className="size-4" aria-hidden />
          Lead baru
        </Button>
      </div>

      {/* Always visible, with counts. On a phone the row scrolls sideways
          instead of wrapping into four lines above the list — the one place
          horizontal scrolling is intended (td.md §4.2). */}
      <div
        role="group"
        aria-label="Filter status"
        className="-mx-3.5 flex gap-2 overflow-x-auto px-3.5 pb-0.5 [scrollbar-width:none] md:mx-0 md:flex-wrap md:overflow-visible md:px-0"
      >
        <button
          type="button"
          aria-pressed={unassignedActive}
          onClick={() => updateFilterParams({ assigned_to: unassignedActive ? null : "none" })}
          className={cn(
            "min-h-9 shrink-0 rounded-full border-[1.5px] border-accent-strong px-3 text-[12.5px] font-bold whitespace-nowrap transition-colors md:min-h-8",
            unassignedActive ? "bg-accent-strong text-white" : "bg-card text-accent-strong"
          )}
        >
          Tanpa pemilik aktif · {summary?.unassigned ?? "…"}
        </button>
        {LEAD_STATUSES.map((status) => {
          const active = statusFilter.includes(status);
          const meta = STATUS_META[status];
          return (
            <button
              key={status}
              type="button"
              aria-pressed={active}
              onClick={() => updateFilterParams({ status: toggleCSVValue(statusFilter, status).join(",") || null })}
              className="min-h-9 shrink-0 rounded-full border-[1.5px] px-3 text-[12.5px] font-semibold whitespace-nowrap transition-colors md:min-h-8"
              // Active = solid status color with white text: the same pair as
              // the badge read backwards, 5.03–5.27:1 (#159).
              style={{
                borderColor: meta.color,
                background: active ? meta.color : "var(--card)",
                color: active ? "#fff" : meta.color,
              }}
            >
              {meta.label} · {statusCount(summary, status)}
            </button>
          );
        })}
      </div>

      <div className="hidden flex-wrap items-center gap-x-4 gap-y-2.5 md:flex">
        {secondaryFilters}
        {hasAnyFilter && (
          <button
            type="button"
            onClick={handleClearFilters}
            className="text-[13px] font-semibold text-accent-strong underline underline-offset-2"
          >
            Hapus semua filter
          </button>
        )}
      </div>
      {hasAnyFilter && (
        <button
          type="button"
          onClick={handleClearFilters}
          className="-mt-1 self-start text-[13px] font-semibold text-accent-strong underline underline-offset-2 md:hidden"
        >
          Hapus semua filter
        </button>
      )}

      {error && <FormErrorBanner message={error} />}

      {loading && leads.length === 0 && !error && (
        <div aria-busy="true" aria-label="Memuat lead" className="flex flex-col gap-2">
          {Array.from({ length: 6 }, (_, i) => (
            <div key={i} className="h-19 animate-pulse rounded-[10px] bg-muted md:h-13" />
          ))}
        </div>
      )}

      {isEmptyNoData && (
        <div className="rounded-xl border border-dashed border-border bg-card px-5 py-14 text-center">
          <div className="mb-1.5 text-[16px] font-bold">Belum ada lead</div>
          <p className="mx-auto mb-4.5 max-w-[36ch] text-[14px] text-muted-foreground">
            Lead akan muncul di sini begitu masuk lewat formulir, API, webhook, atau dicatat manual.
          </p>
          <Button onClick={() => setNewLeadOpen(true)} className="h-10 px-4">
            Tambah lead pertama
          </Button>
        </div>
      )}

      {isEmptyFiltered && (
        <div className="rounded-xl border border-dashed border-border bg-card px-5 py-14 text-center">
          <div className="mb-1.5 text-[16px] font-bold">Tidak ada lead yang cocok</div>
          <p className="mx-auto mb-4.5 max-w-[36ch] text-[14px] text-muted-foreground">
            Coba lebarkan filter status, sumber, pemilik, atau tanggal masuk.
          </p>
          <Button variant="outline" onClick={handleClearFilters} className="h-10 px-4">
            Hapus semua filter
          </Button>
        </div>
      )}

      {showTable && (
        <>
          {/* 768px and up: table. Columns join as width allows, so Nama never
              gets crushed by fixed columns (table-fixed, widths in rem):
                768+   Nama (with #number beneath) · Status · Pemilik · Masuk
                1024+  + Sumber
                1280+  + Nomor as its own column · Kontak
              Status is 12.5rem because "Tidak Memenuhi Syarat" is the widest
              badge and must not be clipped. */}
          <div className="hidden overflow-hidden rounded-xl border border-border bg-card md:block">
            <table className="w-full table-fixed border-collapse text-[14px]">
              <thead>
                <tr className="bg-muted text-left text-[11.5px] font-bold tracking-[0.05em] text-muted-foreground uppercase">
                  <th className="hidden w-24 px-4 py-2.5 xl:table-cell">Nomor</th>
                  <th className="px-4 py-2.5 xl:px-3">Nama</th>
                  <th className="w-[12.5rem] px-3 py-2.5">Status</th>
                  <th className="w-32 px-3 py-2.5 lg:w-36">Pemilik</th>
                  <th className="hidden w-24 px-3 py-2.5 lg:table-cell">Sumber</th>
                  <th className="w-28 px-3 py-2.5">Masuk</th>
                  <th className="hidden px-3 py-2.5 xl:table-cell">Kontak</th>
                </tr>
              </thead>
              <tbody>
                {leads.map((lead) => {
                  const owner = ownerName(lead);
                  return (
                    <tr
                      key={lead.id}
                      onClick={() => router.push(`/leads/${lead.id}`)}
                      className="cursor-pointer border-t border-border/60 hover:bg-muted/50"
                    >
                      <td className="hidden px-4 py-3 font-mono text-[13px] text-muted-foreground xl:table-cell">
                        #{lead.lead_number}
                      </td>
                      <td className="min-w-0 px-4 py-3 xl:px-3">
                        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                          {/* The row is clickable for the mouse; this link is
                              what a keyboard or screen reader reaches. */}
                          <Link
                            href={`/leads/${lead.id}`}
                            onClick={(e) => e.stopPropagation()}
                            className="truncate font-semibold hover:underline"
                          >
                            {lead.name}
                          </Link>
                          {!hasContact(lead) && <NoContactBadge />}
                        </div>
                        <div className="font-mono text-[12.5px] text-muted-foreground xl:hidden">
                          #{lead.lead_number}
                        </div>
                      </td>
                      <td className="px-3 py-3">
                        <StatusBadge status={lead.status} />
                      </td>
                      <td
                        className={cn(
                          "truncate px-3 py-3",
                          owner ? "text-foreground" : "font-semibold text-accent-strong"
                        )}
                      >
                        {owner ?? "Tanpa pemilik"}
                      </td>
                      <td className="hidden px-3 py-3 text-muted-foreground lg:table-cell">
                        {SOURCE_LABELS[lead.source]}
                      </td>
                      <td className="px-3 py-3 whitespace-nowrap text-muted-foreground">
                        {formatDateID(lead.created_at)}
                      </td>
                      <td className="hidden truncate px-3 py-3 text-[13px] text-muted-foreground xl:table-cell">
                        {primaryContact(lead)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* Below 768px: cards. No table that has to be scrolled sideways. */}
          <ul className="flex flex-col gap-2.5 md:hidden">
            {leads.map((lead) => {
              const owner = ownerName(lead);
              return (
                <li key={lead.id}>
                  <Link
                    href={`/leads/${lead.id}`}
                    className="block rounded-[10px] border border-border bg-card p-3.5 active:bg-muted/60"
                  >
                    <div className="flex items-start justify-between gap-2.5">
                      <div className="min-w-0">
                        <div className="truncate text-[15px] font-bold">{lead.name}</div>
                        <div className="mt-0.5 font-mono text-[12.5px] text-muted-foreground">
                          #{lead.lead_number}
                        </div>
                      </div>
                      <StatusBadge status={lead.status} className="shrink-0" />
                    </div>
                    {!hasContact(lead) && <NoContactBadge className="mt-2" />}
                    <div className="mt-2.5 flex flex-wrap gap-x-3.5 gap-y-1 text-[13px] text-muted-foreground">
                      <span className={owner ? undefined : "font-semibold text-accent-strong"}>
                        {owner ?? "Tanpa pemilik"}
                      </span>
                      <span>{SOURCE_LABELS[lead.source]}</span>
                      <span>{formatDateID(lead.created_at)}</span>
                    </div>
                  </Link>
                </li>
              );
            })}
          </ul>

          <div className="flex flex-wrap items-center justify-between gap-2.5 text-[13px] text-muted-foreground">
            <span>
              Menampilkan {rangeStart}–{rangeEnd} dari {total} lead
            </span>
            {totalPages > 1 && (
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  disabled={page <= 1}
                  onClick={() => updateParams({ page: String(page - 1) })}
                  className="h-10 bg-card md:h-8"
                >
                  Sebelumnya
                </Button>
                <span className="hidden sm:inline">
                  Halaman {page} dari {totalPages}
                </span>
                <Button
                  variant="outline"
                  disabled={page >= totalPages}
                  onClick={() => updateParams({ page: String(page + 1) })}
                  className="h-10 bg-card md:h-8"
                >
                  Berikutnya
                </Button>
              </div>
            )}
          </div>
        </>
      )}

      {/* Phone: the create action rides above the bottom bar, in thumb
          reach. Hidden while the list is empty — that state has its own
          "Tambah lead pertama" button and two would compete. */}
      {!isEmptyNoData && (
        <button
          type="button"
          onClick={() => setNewLeadOpen(true)}
          aria-label="Lead baru"
          className="fixed right-4 bottom-[calc(4.75rem+env(safe-area-inset-bottom))] z-30 flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-[0_8px_20px_oklch(0.56_0.19_41/35%)] md:hidden"
        >
          <Plus className="size-6" aria-hidden />
        </button>
      )}

      <Sheet open={filterSheetOpen} onOpenChange={setFilterSheetOpen}>
        <SheetContent>
          <SheetTitle className="text-[16px]">Filter</SheetTitle>
          <div className="flex flex-col gap-4">{secondaryFilters}</div>
          <div className="mt-2 flex gap-2.5">
            <Button
              variant="outline"
              onClick={handleClearFilters}
              disabled={!hasAnyFilter}
              className="h-11 flex-1 text-[14px]"
            >
              Hapus filter
            </Button>
            {/* Filters apply as they're chosen (the list behind the sheet is
                already updated) — this button only closes the sheet. */}
            <Button onClick={() => setFilterSheetOpen(false)} className="h-11 flex-1 text-[14px]">
              Tampilkan {loading ? "…" : total} lead
            </Button>
          </div>
        </SheetContent>
      </Sheet>

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
