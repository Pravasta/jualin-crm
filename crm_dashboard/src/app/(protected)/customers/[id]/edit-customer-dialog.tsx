"use client";

// Same shape as leads/[id]/edit-lead-dialog.tsx, minus `version` — a
// customer has no optimistic-locking field to carry (lib/customers.ts's
// doc comment: no offline mobile write path for customers), so there's
// no conflict case to route to a ConflictDialog here.
import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FormErrorBanner } from "@/components/form-error-banner";
import { updateCustomer } from "@/lib/customers";
import type { Customer } from "@/lib/leads";
import { globalMessage } from "@/lib/auth-errors";

export function EditCustomerDialog({
  open,
  onOpenChange,
  customer,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  customer: Customer;
  onSaved: (updated: Customer) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {/* Remounted per customer.id — same "reset form on prop change
            via key" pattern as EditLeadDialog, avoiding a
            react-hooks/set-state-in-effect violation. No `.version` to
            include in the key since customers don't have one. */}
        <EditCustomerForm
          key={customer.id}
          customer={customer}
          onOpenChange={onOpenChange}
          onSaved={onSaved}
        />
      </DialogContent>
    </Dialog>
  );
}

function EditCustomerForm({
  customer,
  onOpenChange,
  onSaved,
}: {
  customer: Customer;
  onOpenChange: (open: boolean) => void;
  onSaved: (updated: Customer) => void;
}) {
  const [name, setName] = useState(customer.name);
  const [email, setEmail] = useState(customer.email ?? "");
  const [phone, setPhone] = useState(customer.phone ?? "");
  const [company, setCompany] = useState(customer.company ?? "");
  const [notes, setNotes] = useState(customer.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const updated = await updateCustomer(customer.id, {
        name,
        email: email || null,
        phone: phone || null,
        company: company || null,
        notes: notes || null,
      });
      onOpenChange(false);
      onSaved(updated);
    } catch (err) {
      setError(globalMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Ubah customer</DialogTitle>
        <DialogDescription>
          Perubahan ini tidak mengubah lead asalnya — keduanya tersimpan terpisah.
        </DialogDescription>
      </DialogHeader>

      <form onSubmit={handleSubmit} className="flex flex-col gap-3">
        <FormErrorBanner message={error} />

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="customer-name">Nama</Label>
          <Input
            id="customer-name"
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="customer-email">Email</Label>
          <Input
            id="customer-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="customer-phone">Telepon</Label>
          <Input
            id="customer-phone"
            type="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="customer-company">Perusahaan</Label>
          <Input
            id="customer-company"
            value={company}
            onChange={(e) => setCompany(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="customer-notes">Catatan</Label>
          <Textarea id="customer-notes" value={notes} onChange={(e) => setNotes(e.target.value)} />
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Batal
          </Button>
          <Button type="submit" disabled={loading}>
            {loading ? "Menyimpan…" : "Simpan"}
          </Button>
        </DialogFooter>
      </form>
    </>
  );
}
