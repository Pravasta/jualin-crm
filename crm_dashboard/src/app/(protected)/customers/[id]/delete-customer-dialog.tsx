"use client";

// Same shape as leads/[id]/delete-lead-dialog.tsx. Unlike deactivating a
// membership (#34), deleting a customer has no dependent-data branch on
// the backend (internal/customer/usecase.go's Delete is a plain
// tenant-scoped soft delete) — a single confirm is the correct amount
// of friction here, not a multi-step dialog.
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
import { globalMessage } from "@/lib/auth-errors";

export function DeleteCustomerDialog({
  open,
  onOpenChange,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => Promise<void>;
}) {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await onConfirm();
    } catch (err) {
      setError(globalMessage(err));
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="border-t-4 border-t-destructive">
        <DialogHeader>
          <DialogTitle>Hapus customer ini?</DialogTitle>
          <DialogDescription>
            Tindakan ini tidak bisa dibatalkan. Lead asal yang pernah dikonversi menjadi customer
            ini tidak ikut terhapus.
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
            {loading ? "Menghapus…" : "Ya, hapus customer"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
