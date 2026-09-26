"use client";

// Built from the design's CUSTOMERS section, extended beyond it: the
// mockup has neither search nor pagination controls at all
// (sectionIsCustomers is even `false` there — less fleshed out than
// LEADS LIST/TEAM) even though the checklist requires both. Row click
// navigates to a route (/customers/{id}) rather than the mockup's
// centered modal, matching how /leads/{id} already works elsewhere in
// this app — one detail-screen convention, not two.
//
// Phase 8.6 (#164): table from 768px, cards below. The handoff's "Nilai
// kontrak" column is dummy data — no amount exists on a customer, and money
// figures are out of scope (brief §6) — so it is not here.
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams, usePathname } from "next/navigation";
import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { FormErrorBanner } from "@/components/form-error-banner";
import { listCustomers } from "@/lib/customers";
import type { Customer } from "@/lib/leads";
import { formatDateID } from "@/lib/date";
import { globalMessage } from "@/lib/auth-errors";
import { useDebouncedValue } from "@/lib/use-debounced-value";

const PER_PAGE = 25;

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function CustomerList() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const urlKeyword = searchParams.get("q") ?? "";
  const page = Math.max(1, Number(searchParams.get("page")) || 1);

  const [keywordInput, setKeywordInput] = useState(urlKeyword);
  const debouncedKeyword = useDebouncedValue(keywordInput, 300);

  const [customers, setCustomers] = useState<Customer[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [loadedKey, setLoadedKey] = useState<string | null>(null);

  function updateParams(patch: Record<string, string | null>) {
    const params = new URLSearchParams(searchParams.toString());
    for (const [key, value] of Object.entries(patch)) {
      if (value === null || value === "") params.delete(key);
      else params.set(key, value);
    }
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  }

  useEffect(() => {
    if (debouncedKeyword !== urlKeyword) {
      updateParams({ q: debouncedKeyword || null, page: null });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedKeyword]);

  const requestKey = JSON.stringify([urlKeyword, page]);
  const loading = loadedKey !== requestKey;

  useEffect(() => {
    const controller = new AbortController();
    listCustomers({ q: urlKeyword || undefined, page, perPage: PER_PAGE }, controller.signal)
      .then(({ data, meta }) => {
        setCustomers(data);
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
  }, [urlKeyword, page]);

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const rangeStart = total === 0 ? 0 : (page - 1) * PER_PAGE + 1;
  const rangeEnd = Math.min(page * PER_PAGE, total);
  const isEmptyNoData = !loading && total === 0 && !urlKeyword;
  const isEmptyFiltered = !loading && total === 0 && !!urlKeyword;

  return (
    <div className="flex w-full flex-col gap-3.5 md:gap-4">
      <div className="relative md:max-w-96">
        <Search
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden
        />
        <Input
          value={keywordInput}
          onChange={(e) => setKeywordInput(e.target.value)}
          placeholder="Cari nama, email, atau telepon"
          aria-label="Cari customer"
          className="h-11 bg-card pl-9 md:h-9"
        />
      </div>

      {error && <FormErrorBanner message={error} />}

      {loading && customers.length === 0 && !error && (
        <div aria-busy="true" aria-label="Memuat customer" className="flex flex-col gap-2">
          {Array.from({ length: 5 }, (_, i) => (
            <div key={i} className="h-17 animate-pulse rounded-[10px] bg-muted md:h-12" />
          ))}
        </div>
      )}

      {isEmptyNoData && (
        <div className="rounded-xl border border-dashed border-border bg-card px-5 py-14 text-center">
          <div className="mb-1.5 text-[16px] font-bold">Belum ada customer</div>
          <p className="mx-auto max-w-[38ch] text-[14px] text-muted-foreground">
            Customer muncul di sini setelah sebuah lead berstatus Menang dikonversi.
          </p>
        </div>
      )}

      {isEmptyFiltered && (
        <div className="rounded-xl border border-dashed border-border bg-card px-5 py-14 text-center">
          <div className="mb-1.5 text-[16px] font-bold">Tidak ada customer yang cocok</div>
          <p className="mx-auto mb-4.5 max-w-[36ch] text-[14px] text-muted-foreground">
            Tidak ada customer yang sesuai dengan kata kunci ini.
          </p>
          <Button variant="outline" onClick={() => setKeywordInput("")} className="h-10 px-4">
            Hapus pencarian
          </Button>
        </div>
      )}

      {!loading && total > 0 && (
        <>
          <div className="hidden overflow-hidden rounded-xl border border-border bg-card md:block">
            <table className="w-full table-fixed border-collapse text-[14px]">
              <thead>
                <tr className="bg-muted text-left text-[11.5px] font-bold tracking-[0.05em] text-muted-foreground uppercase">
                  <th className="px-4 py-2.5">Customer</th>
                  <th className="hidden px-3 py-2.5 lg:table-cell">Perusahaan</th>
                  <th className="px-3 py-2.5">Kontak</th>
                  <th className="w-36 px-3 py-2.5">Customer sejak</th>
                </tr>
              </thead>
              <tbody>
                {customers.map((customer) => (
                  <tr
                    key={customer.id}
                    onClick={() => router.push(`/customers/${customer.id}`)}
                    className="cursor-pointer border-t border-border/60 hover:bg-muted/50"
                  >
                    <td className="truncate px-4 py-3">
                      {/* Keyboard/screen-reader path; the row is the mouse path. */}
                      <Link
                        href={`/customers/${customer.id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="font-semibold hover:underline"
                      >
                        {customer.name}
                      </Link>
                    </td>
                    <td className="hidden truncate px-3 py-3 text-muted-foreground lg:table-cell">
                      {customer.company?.trim() || "—"}
                    </td>
                    <td className="truncate px-3 py-3 text-[13px] text-muted-foreground">
                      {contactLine(customer)}
                    </td>
                    <td className="px-3 py-3 whitespace-nowrap text-muted-foreground">
                      {formatDateID(customer.converted_at)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <ul className="flex flex-col gap-2.5 md:hidden">
            {customers.map((customer) => (
              <li key={customer.id}>
                <Link
                  href={`/customers/${customer.id}`}
                  className="block rounded-[10px] border border-border bg-card p-3.5 active:bg-muted/60"
                >
                  <div className="truncate text-[15px] font-bold">{customer.name}</div>
                  {customer.company?.trim() && (
                    <div className="truncate text-[13px] text-muted-foreground">{customer.company}</div>
                  )}
                  <div className="mt-2 flex flex-wrap gap-x-3.5 gap-y-1 text-[13px] text-muted-foreground">
                    <span className="[overflow-wrap:anywhere]">{contactLine(customer)}</span>
                    <span>Sejak {formatDateID(customer.converted_at)}</span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>

          <div className="flex flex-wrap items-center justify-between gap-2.5 text-[13px] text-muted-foreground">
            <span>
              Menampilkan {rangeStart}–{rangeEnd} dari {total} customer
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
    </div>
  );
}

// Phone and email together — a customer is reached either way, and unlike
// the lead table there is room (#164). Blank-safe via trim.
function contactLine(customer: Customer): string {
  return [customer.phone, customer.email].map((v) => v?.trim()).filter(Boolean).join(" · ") || "—";
}
