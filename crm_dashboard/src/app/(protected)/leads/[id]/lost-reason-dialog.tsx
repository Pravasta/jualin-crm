"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FormErrorBanner } from "@/components/form-error-banner";
import { LOST_REASON_LABELS, LOST_REASONS, type LostReason } from "@/lib/labels";

// lost_reason is mandatory the moment status becomes "lost" (TD phase 2
// §5, ck_leads_lost_requires_reason) — this dialog is the only path to
// that transition, so "lost" without a reason literally cannot be sent
// from this screen (issue #33 acceptance criterion).
export function LostReasonDialog({
  open,
  onOpenChange,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: LostReason) => Promise<void>;
}) {
  const [reason, setReason] = useState<LostReason | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleConfirm() {
    if (!reason) return;
    setLoading(true);
    setError(null);
    try {
      await onConfirm(reason);
      setReason(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Terjadi kesalahan.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setReason(null);
        onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Tandai sebagai Kalah</DialogTitle>
          <DialogDescription>Pilih alasan lead ini kalah.</DialogDescription>
        </DialogHeader>

        <FormErrorBanner message={error} />

        <div className="flex flex-col gap-1.5">
          {LOST_REASONS.map((r) => (
            <button
              key={r}
              type="button"
              aria-pressed={reason === r}
              onClick={() => setReason(r)}
              className={cn(
                "min-h-11 rounded-lg border-[1.5px] px-3 text-left text-sm transition-colors md:min-h-9",
                reason === r
                  ? "border-destructive bg-destructive/6 font-semibold text-destructive"
                  : "border-border bg-card hover:bg-muted"
              )}
            >
              {LOST_REASON_LABELS[r]}
            </button>
          ))}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Batal
          </Button>
          <Button
            type="button"
            disabled={!reason || loading}
            onClick={handleConfirm}
            className="bg-destructive text-white hover:bg-destructive/90"
          >
            {loading ? "Menyimpan…" : "Tandai kalah"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
