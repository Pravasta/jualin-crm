"use client";

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

export function DeleteLeadDialog({
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
      setError(err instanceof Error ? err.message : "Terjadi kesalahan.");
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="border-t-4 border-t-destructive">
        <DialogHeader>
          <DialogTitle>Hapus lead ini?</DialogTitle>
          <DialogDescription>
            Tindakan ini tidak bisa dibatalkan. Seluruh riwayat, catatan, dan task pada lead ini
            akan ikut terhapus.
          </DialogDescription>
        </DialogHeader>
        <FormErrorBanner message={error} />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Batal
          </Button>
          {/* button.tsx's "destructive" variant is a soft/light style
              (text-destructive on a tinted bg) — this needs the solid
              red-with-white-text treatment the design gives a truly
              destructive confirm, so it's styled directly rather than
              introducing a second destructive variant for one button. */}
          <Button
            type="button"
            disabled={loading}
            onClick={handleConfirm}
            className="bg-destructive text-white hover:bg-destructive/85"
          >
            {loading ? "Menghapus…" : "Ya, hapus lead"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
