"use client";

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
import { updateLead, type Lead } from "@/lib/leads";
import { globalMessage, versionConflictCurrent } from "@/lib/auth-errors";

// The design's LEAD DETAIL section never surfaces an edit form for
// name/email/phone/company/notes at all — only status/assignment/notes-
// as-timeline-entries. The checklist is explicit ("Ubah field umum —
// PATCH /v1/leads/{id}, membawa version"), so this modal exists to cover
// it; not a redesign of anything the mockup showed.
export function EditLeadDialog({
  open,
  onOpenChange,
  lead,
  onSaved,
  onConflict,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  lead: Lead;
  onSaved: (updated: Lead) => void;
  onConflict: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {/* Keyed by lead.id + version so the form REMOUNTS — with fresh
            useState initializers — every time the dialog opens against a
            possibly-different `lead` (e.g. after a conflict reload
            elsewhere on the screen changed it while this was closed).
            That's the React-recommended way to reset state on a prop
            change; a useEffect calling setState synchronously is what
            react-hooks/set-state-in-effect (React 19 toolchain) forbids. */}
        <EditLeadForm
          key={`${lead.id}-${lead.version}`}
          lead={lead}
          onOpenChange={onOpenChange}
          onSaved={onSaved}
          onConflict={onConflict}
        />
      </DialogContent>
    </Dialog>
  );
}

function EditLeadForm({
  lead,
  onOpenChange,
  onSaved,
  onConflict,
}: {
  lead: Lead;
  onOpenChange: (open: boolean) => void;
  onSaved: (updated: Lead) => void;
  onConflict: () => void;
}) {
  const [name, setName] = useState(lead.name);
  const [email, setEmail] = useState(lead.email ?? "");
  const [phone, setPhone] = useState(lead.phone ?? "");
  const [company, setCompany] = useState(lead.company ?? "");
  const [notes, setNotes] = useState(lead.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const updated = await updateLead(lead.id, {
        version: lead.version,
        name,
        email: email || null,
        phone: phone || null,
        company: company || null,
        notes: notes || null,
      });
      onOpenChange(false);
      onSaved(updated);
    } catch (err) {
      if (versionConflictCurrent<Lead>(err)) {
        onOpenChange(false);
        onConflict();
      } else {
        setError(globalMessage(err));
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Ubah lead</DialogTitle>
        <DialogDescription>Perbarui data kontak lead ini.</DialogDescription>
      </DialogHeader>

      <form onSubmit={handleSubmit} className="flex flex-col gap-3">
        <FormErrorBanner message={error} />

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="edit-name">Nama</Label>
          <Input id="edit-name" required value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="edit-email">Email</Label>
          <Input
            id="edit-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="edit-phone">Telepon</Label>
          <Input id="edit-phone" type="tel" value={phone} onChange={(e) => setPhone(e.target.value)} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="edit-company">Perusahaan</Label>
          <Input id="edit-company" value={company} onChange={(e) => setCompany(e.target.value)} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="edit-notes">Catatan</Label>
          <Textarea id="edit-notes" value={notes} onChange={(e) => setNotes(e.target.value)} />
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
