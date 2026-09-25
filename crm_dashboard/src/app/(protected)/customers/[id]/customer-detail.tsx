"use client";

// The design's CUSTOMERS detail is a centered modal with a dead '#'
// link back to the originating lead and no edit form at all (only
// delete) — this is a route instead (see customer-list.tsx), and adds
// the edit form the checklist requires (`PATCH /v1/customers/{id}`).
// The "from lead" link is real here: the design's fake customer objects
// carry their own `fromLeadNumber` seed field, but the actual Customer
// JSON only has `converted_from_lead_id` (a UUID) — the lead itself is
// fetched to show its current name/number, which is also what proves
// on screen that editing this customer never touched the lead it came
// from (AC: "mengubah nama customer tidak mengubah lead asalnya").
//
// Phase 8.6 (#164): same header pattern as the lead detail (#162). The
// handoff's contract value, order history and customer timeline are dummy
// data — a customer carries none of them, and money is out of scope — so
// they are not drawn.
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { FormErrorBanner } from "@/components/form-error-banner";
import { ApiError } from "@/lib/api-types";
import { deleteCustomer, getCustomer } from "@/lib/customers";
import { getLead, type Customer, type Lead } from "@/lib/leads";
import { formatDateID } from "@/lib/date";
import { globalMessage } from "@/lib/auth-errors";
import { useSession } from "@/lib/session-context";
import { EditCustomerDialog } from "./edit-customer-dialog";
import { DeleteCustomerDialog } from "./delete-customer-dialog";

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function CustomerDetail({ customerId }: { customerId: string }) {
  const router = useRouter();
  const session = useSession();

  const [customer, setCustomer] = useState<Customer | null>(null);
  const [fromLead, setFromLead] = useState<Lead | null>(null);
  const [fromLeadMissing, setFromLeadMissing] = useState(false);
  const [notFound, setNotFound] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    getCustomer(customerId, controller.signal)
      .then((data) => {
        setCustomer(data);
        setNotFound(false);
        setLoadError(null);
        // Best-effort: a converted lead is virtually always still there
        // (leads aren't deleted by conversion), but if it was separately
        // deleted since, the link degrades to plain text rather than
        // failing the whole screen.
        getLead(data.converted_from_lead_id, controller.signal)
          .then((lead) => setFromLead(lead))
          .catch((err) => {
            if (isAbortError(err)) return;
            setFromLeadMissing(true);
          });
      })
      .catch((err) => {
        if (isAbortError(err)) return;
        if (err instanceof ApiError && err.code === "not_found") setNotFound(true);
        else setLoadError(globalMessage(err));
      });
    return () => controller.abort();
  }, [customerId]);

  if (notFound) {
    return (
      <div className="flex flex-col items-center gap-3 py-16 text-center">
        <p className="text-sm text-muted-foreground">Customer tidak ditemukan.</p>
        <Button variant="outline" onClick={() => router.push("/customers")}>
          Kembali ke daftar customer
        </Button>
      </div>
    );
  }

  if (loadError) {
    return <FormErrorBanner message={loadError} />;
  }

  if (!customer) {
    return <div className="py-16 text-center text-sm text-muted-foreground">Memuat…</div>;
  }

  // ActionCustomerUpdate/Delete are Owner/Admin only — Manager (and
  // Employee) get read-only (docs/architecture/authorization.md). The
  // button is withheld here; if the backend still rejects it (e.g. role
  // changed in another tab), the error shows apa adanya, same as #34.
  const canManage = session.role === "owner" || session.role === "admin";

  const facts: { label: string; value: string }[] = [
    { label: "Email", value: customer.email?.trim() || "—" },
    { label: "Telepon", value: customer.phone?.trim() || "—" },
    { label: "Perusahaan", value: customer.company?.trim() || "—" },
    { label: "Customer sejak", value: formatDateID(customer.converted_at) },
  ];

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-3.5">
      <button
        type="button"
        onClick={() => router.push("/customers")}
        className="flex min-h-9 items-center gap-1.5 self-start text-[13.5px] font-medium text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Daftar customer
      </button>

      <Card>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-start justify-between gap-3">
            <h1 className="min-w-0 text-[20px] leading-tight font-bold tracking-tight break-words md:text-[24px]">
              {customer.name}
            </h1>
            {canManage && (
              <Button variant="outline" size="sm" onClick={() => setEditOpen(true)} className="h-9 shrink-0 md:h-7">
                Ubah
              </Button>
            )}
          </div>

          {/* Values wrap, never truncate — same rule as the lead detail (#162). */}
          <dl className="grid grid-cols-2 gap-x-4 gap-y-3 md:grid-cols-4">
            {facts.map((f) => (
              <div key={f.label} className="min-w-0">
                <dt className="text-[11.5px] font-semibold tracking-[0.04em] text-muted-foreground uppercase">
                  {f.label}
                </dt>
                <dd className="mt-0.5 text-[14px] [overflow-wrap:anywhere]">{f.value}</dd>
              </div>
            ))}
          </dl>

          {customer.notes && (
            <p className="rounded-lg bg-muted px-3 py-2 text-[13.5px] whitespace-pre-line">{customer.notes}</p>
          )}

          <div className="border-t border-border pt-3.5 text-[14px]">
            <span className="text-muted-foreground">Berasal dari lead </span>
            {fromLead ? (
              <Link
                href={`/leads/${fromLead.id}`}
                className="font-semibold text-accent-strong underline underline-offset-2"
              >
                <span className="font-mono">#{fromLead.lead_number}</span> {fromLead.name}
              </Link>
            ) : fromLeadMissing ? (
              <span className="text-muted-foreground">yang sudah dihapus</span>
            ) : (
              <span className="text-muted-foreground">…</span>
            )}
          </div>
        </CardContent>
      </Card>

      {canManage && (
        // Destructive, and looks it (brief §5.1 principle 3) — outside the
        // card, apart from the everyday actions.
        <Button
          type="button"
          variant="outline"
          onClick={() => setDeleteOpen(true)}
          className="h-11 self-start border-destructive/40 bg-card text-destructive hover:bg-destructive/6 hover:text-destructive md:h-9"
        >
          <Trash2 className="size-4" aria-hidden />
          Hapus customer
        </Button>
      )}

      <EditCustomerDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        customer={customer}
        onSaved={(updated) => {
          setCustomer(updated);
          setEditOpen(false);
        }}
      />
      <DeleteCustomerDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onConfirm={async () => {
          await deleteCustomer(customer.id);
          router.push("/customers");
        }}
      />
    </div>
  );
}
