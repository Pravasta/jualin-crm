"use client";

// Built from the design's CUSTOMERS section, extended beyond it: the
// mockup has neither search nor pagination controls at all
// (sectionIsCustomers is even `false` there — less fleshed out than
// LEADS LIST/TEAM) even though the checklist requires both. Row click
// navigates to a route (/customers/{id}) rather than the mockup's
// centered modal, matching how /leads/{id} already works elsewhere in
// this app — one detail-screen convention, not two.
import { useEffect, useState } from "react";
import { useRouter, useSearchParams, usePathname } from "next/navigation";
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
    <div>
      <div className="mb-3.5">
        <Input
          value={keywordInput}
          onChange={(e) => setKeywordInput(e.target.value)}
          placeholder="Cari nama, email, atau telepon…"
          className="h-8.5 max-w-80"
        />
      </div>

      {error && <FormErrorBanner message={error} />}

      {isEmptyNoData && (
        <div className="rounded-lg border border-dashed border-muted-foreground/30 bg-background px-6 py-12 text-center">
          <div className="mb-1.5 text-[14.5px] font-semibold">Belum ada customer</div>
          <div className="text-[13px] text-muted-foreground">
            Customer muncul di sini setelah sebuah lead dikonversi.
          </div>
        </div>
      )}

      {isEmptyFiltered && (
        <div className="rounded-lg border border-dashed border-muted-foreground/30 bg-background px-6 py-12 text-center">
          <div className="mb-1.5 text-[14.5px] font-semibold">Tidak ada customer yang cocok</div>
          <div className="text-[13px] text-muted-foreground">
            Tidak ada customer yang sesuai dengan kata kunci saat ini.
          </div>
        </div>
      )}

      {loading && customers.length === 0 && !error && (
        <div className="rounded-lg border border-border bg-background px-6 py-12 text-center text-sm text-muted-foreground">
          Memuat…
        </div>
      )}

      {!loading && total > 0 && (
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          <div className="overflow-x-auto">
            <table className="w-full border-collapse">
              <thead>
                <tr className="bg-muted/40">
                  <th className="px-3.5 py-2 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Customer
                  </th>
                  <th className="px-3.5 py-2 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Kontak
                  </th>
                  <th className="px-3.5 py-2 text-left text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Pelanggan sejak
                  </th>
                </tr>
              </thead>
              <tbody>
                {customers.map((customer) => (
                  <tr
                    key={customer.id}
                    onClick={() => router.push(`/customers/${customer.id}`)}
                    className="cursor-pointer border-t border-border/70 hover:bg-muted/40"
                  >
                    <td className="px-3.5 py-2.5 text-[13px] font-medium">{customer.name}</td>
                    <td className="px-3.5 py-2.5 text-[13px] text-foreground/70">
                      {[customer.email, customer.phone].filter(Boolean).join(" · ") || "—"}
                    </td>
                    <td className="px-3.5 py-2.5 text-[13px] text-foreground/70">
                      {formatDateID(customer.converted_at)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="flex items-center justify-between border-t border-border px-3.5 py-2.5 text-[12.5px] text-muted-foreground">
            <span>
              Menampilkan {rangeStart}–{rangeEnd} dari {total} customer
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
    </div>
  );
}
