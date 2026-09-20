"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FieldError } from "@/components/field-error";
import { FormErrorBanner } from "@/components/form-error-banner";
import { createLead } from "@/lib/leads";
import { hasContact, shouldConfirmNoContact } from "@/lib/lead-contact";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";
import { isPlanQuotaExceeded } from "@/lib/plan";

export function NewLeadDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}) {
  const router = useRouter();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  // True once the "belum ada kontak" warning has been shown. Cleared whenever
  // email or phone changes, so the question is always about what is in the
  // form NOW — and never a block: "Tetap simpan" is the same submit.
  const [warned, setWarned] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  // Set only on a REAL 403 plan_quota_exceeded (subscription #123) — the
  // link to /subscription only makes sense in that specific case, not
  // for every other error this dialog can show (issue #125 AC).
  const [quotaExceeded, setQuotaExceeded] = useState(false);
  const [loading, setLoading] = useState(false);

  function reset() {
    setName("");
    setEmail("");
    setPhone("");
    setWarned(false);
    setFieldErrors({});
    setError(null);
    setQuotaExceeded(false);
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    // A lead made by hand with no contact is asked about ONCE. Leads from a
    // form or the API are never asked anything — nobody is there to ask, and
    // refusing them would throw a customer away (freeze.md).
    if (shouldConfirmNoContact({ email, phone }, warned)) {
      setWarned(true);
      return;
    }
    setError(null);
    setFieldErrors({});
    setQuotaExceeded(false);
    setLoading(true);
    try {
      await createLead({ name, email: email || undefined, phone: phone || undefined });
      reset();
      onOpenChange(false);
      onCreated();
    } catch (err) {
      const fields = fieldErrorsFrom(err);
      if (Object.keys(fields).length > 0) {
        setFieldErrors(fields);
      } else {
        setError(globalMessage(err));
        setQuotaExceeded(isPlanQuotaExceeded(err));
      }
    } finally {
      setLoading(false);
    }
  }

  function backToContact() {
    setWarned(false);
    document.getElementById("lead-email")?.focus();
  }

  function goToSubscription() {
    onOpenChange(false);
    router.push("/subscription");
  }

  // Derived, not stored: typing a contact clears `warned`, and even if it did
  // not, a warning about a form that now HAS a contact would be wrong.
  const warningVisible = warned && !hasContact({ email, phone });

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Lead baru</DialogTitle>
          <DialogDescription>Isi data kontak untuk lead ini.</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <FormErrorBanner message={error} />
          {quotaExceeded && (
            <button
              type="button"
              onClick={goToSubscription}
              className="-mt-1.5 self-start text-[12.5px] text-accent-strong underline"
            >
              Lihat paket & pemakaian
            </button>
          )}

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lead-name">Nama</Label>
            <Input
              id="lead-name"
              required
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <FieldError message={fieldErrors.name} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lead-email">Email</Label>
            <Input
              id="lead-email"
              type="email"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                setWarned(false);
              }}
            />
            <FieldError message={fieldErrors.email} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lead-phone">Telepon</Label>
            <Input
              id="lead-phone"
              type="tel"
              value={phone}
              onChange={(e) => {
                setPhone(e.target.value);
                setWarned(false);
              }}
            />
            <FieldError message={fieldErrors.phone} />
          </div>

          {warningVisible && (
            // Neutral, not an error: a lead without contact is legitimate. The
            // point is that the person creating it knows, not that they erred.
            <div role="alert" className="rounded-md border border-border bg-muted px-3 py-2 text-[13px]">
              Lead ini belum punya kontak dan tidak bisa ditindaklanjuti. Tetap simpan?
            </div>
          )}

          <DialogFooter>
            {warningVisible ? (
              <>
                <Button type="button" variant="outline" onClick={backToContact}>
                  Kembali isi kontak
                </Button>
                <Button type="submit" disabled={loading}>
                  {loading ? "Menyimpan…" : "Tetap simpan"}
                </Button>
              </>
            ) : (
              <>
                <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                  Batal
                </Button>
                <Button type="submit" disabled={loading}>
                  {loading ? "Menyimpan…" : "Buat lead"}
                </Button>
              </>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
