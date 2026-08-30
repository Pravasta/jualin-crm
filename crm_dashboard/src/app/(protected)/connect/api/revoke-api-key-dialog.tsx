"use client";

// Same shape as customers/[id]/delete-customer-dialog.tsx — a single
// confirm is the right amount of friction here. Unlike deactivating a
// membership (#34), revoking an api_key has no dependent-data branch on
// the backend (apikey.Usecase.Revoke is unconditional once authz and
// existence pass) — there's no "409, choose what to do with X" case to
// react to.
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
import { revokeAPIKey, type APIKey } from "@/lib/api-keys";

export function RevokeAPIKeyDialog({
  apiKey,
  onClose,
  onRevoked,
}: {
  apiKey: APIKey | null;
  onClose: () => void;
  onRevoked: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleConfirm() {
    if (!apiKey) return;
    setLoading(true);
    setError(null);
    try {
      await revokeAPIKey(apiKey.id);
      onRevoked();
    } catch (err) {
      setError(globalMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={apiKey !== null} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="border-t-4 border-t-destructive">
        <DialogHeader>
          <DialogTitle>Cabut kunci &ldquo;{apiKey?.name}&rdquo;?</DialogTitle>
          <DialogDescription>
            Tindakan ini tidak bisa dibatalkan. Sistem mana pun yang masih memakai kunci ini akan
            langsung ditolak — pastikan integrasinya sudah tidak dipakai, atau sudah dipindah ke kunci
            lain.
          </DialogDescription>
        </DialogHeader>
        <FormErrorBanner message={error} />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            Batal
          </Button>
          <Button
            type="button"
            disabled={loading}
            onClick={handleConfirm}
            className="bg-destructive text-white hover:bg-destructive/85"
          >
            {loading ? "Mencabut…" : "Ya, cabut kunci"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
