"use client";

// Deactivating a form is a soft delete on the backend (form.Usecase.Delete
// → deleted_at). The public_key stops resolving immediately, so any page
// that still embeds this form will show a 404 instead of the form —
// same "make sure nothing still depends on it" warning as revoking an
// API key. There is no dependent-data branch (unlike deactivating a
// membership, #34): the form has no open leads of its own to reassign.
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FormErrorBanner } from "@/components/form-error-banner";
import { deleteForm } from "@/lib/forms";
import { globalMessage } from "@/lib/auth-errors";

export function DeactivateFormDialog({
  formId,
  formName,
  open,
  onOpenChange,
  onDeactivated,
}: {
  formId: string;
  formName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeactivated: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await deleteForm(formId);
      onDeactivated();
    } catch (err) {
      setError(globalMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="border-t-4 border-t-destructive">
        <DialogHeader>
          <DialogTitle>Nonaktifkan formulir &ldquo;{formName}&rdquo;?</DialogTitle>
          <DialogDescription>
            Formulir yang masih terpasang di situs mana pun akan berhenti tampil. Kampanye atau
            integrasi lain tidak terpengaruh — hanya formulir ini yang dinonaktifkan.
          </DialogDescription>
        </DialogHeader>
        <FormErrorBanner message={error} />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Batal
          </Button>
          <Button
            type="button"
            disabled={loading}
            onClick={handleConfirm}
            className="bg-destructive text-white hover:bg-destructive/85"
          >
            {loading ? "Menonaktifkan…" : "Ya, nonaktifkan"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
