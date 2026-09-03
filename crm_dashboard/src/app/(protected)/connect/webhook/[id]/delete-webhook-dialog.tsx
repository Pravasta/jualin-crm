"use client";

// Deleting an endpoint is a soft delete on the backend
// (webhook.Usecase.Delete → deleted_at). Deliveries already queued for it
// keep their rows — the history stays readable — but no NEW delivery is
// ever enqueued for it again, because Enqueuer's SELECT filters on
// deleted_at IS NULL.
//
// The warning below says exactly that, because "menghapus endpoint" could
// otherwise be read as "and stop the ones in flight", which it does not.
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
import { deleteWebhookEndpoint } from "@/lib/webhooks";
import { globalMessage } from "@/lib/auth-errors";

export function DeleteWebhookDialog({
  endpointId,
  endpointUrl,
  open,
  onOpenChange,
  onDeleted,
}: {
  endpointId: string;
  endpointUrl: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await deleteWebhookEndpoint(endpointId);
      onDeleted();
    } catch (err) {
      setError(globalMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Hapus endpoint?</DialogTitle>
          <DialogDescription>
            Jualin berhenti mengirim event ke <span className="font-mono">{endpointUrl}</span>.
            Riwayat pengiriman yang sudah ada tetap bisa dibaca, tetapi endpoint ini tidak bisa
            dikembalikan — dan secret-nya tidak bisa dipakai lagi.
          </DialogDescription>
        </DialogHeader>

        <FormErrorBanner message={error} />

        {/* Nonaktifkan is the reversible middle ground, and most people
            reaching this dialog actually want it. Saying so here is
            cheaper than an undo that does not exist. */}
        <p className="text-[12.5px] text-muted-foreground">
          Kalau hanya ingin berhenti sementara, tutup dialog ini dan pakai{" "}
          <span className="font-medium">Nonaktifkan</span> — endpoint dan secret-nya tetap tersimpan.
        </p>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Batal
          </Button>
          <Button type="button" variant="destructive" disabled={loading} onClick={handleConfirm}>
            {loading ? "Menghapus…" : "Hapus endpoint"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
