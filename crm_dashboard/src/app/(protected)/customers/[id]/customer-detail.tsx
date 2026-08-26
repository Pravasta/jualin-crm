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
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
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

  return (
    <div>
      <button
        type="button"
        onClick={() => router.push("/customers")}
        className="mb-3.5 flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground"
      >
        ← Kembali ke daftar customer
      </button>

      <div className="max-w-xl">
        <Card>
          <CardContent>
            <div className="mb-1 flex items-start justify-between">
              <div className="flex items-center gap-2 text-[19px] font-semibold">
                {customer.name}
                {canManage && (
                  <button
                    type="button"
                    onClick={() => setEditOpen(true)}
                    className="text-xs font-normal text-accent-strong underline"
                  >
                    Ubah
                  </button>
                )}
              </div>
            </div>
            <div className="mb-2.5 text-[13px] text-muted-foreground">
              Pelanggan sejak {formatDateID(customer.converted_at)}
            </div>

            <div className="flex flex-col gap-1 text-[13px] text-foreground/80">
              {customer.email && <span>{customer.email}</span>}
              {customer.phone && <span>{customer.phone}</span>}
              {customer.company && <span>{customer.company}</span>}
            </div>
            {customer.notes && (
              <div className="mt-2.5 text-[13px] text-muted-foreground">{customer.notes}</div>
            )}

            <div className="mt-3.5 border-t border-border pt-3 text-[13px]">
              Berasal dari lead:{" "}
              {fromLead ? (
                <Link
                  href={`/leads/${fromLead.id}`}
                  className="font-medium text-accent-strong underline"
                >
                  #{fromLead.lead_number} {fromLead.name}
                </Link>
              ) : fromLeadMissing ? (
                <span className="text-muted-foreground">Lead sudah dihapus</span>
              ) : (
                <span className="text-muted-foreground">Memuat…</span>
              )}
            </div>

            {canManage && (
              <div className="mt-4 border-t border-border pt-3.5">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setDeleteOpen(true)}
                  className="border-destructive/35 bg-destructive/6 text-destructive hover:bg-destructive/10"
                >
                  Hapus customer
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

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
