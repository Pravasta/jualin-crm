"use client";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

// Aturan #35 / issue #33's headline acceptance criterion: a save
// conflict must show the conflict, reload the current state, and NEVER
// auto-overwrite. This dialog is the only response to
// version_conflict — every write path in lead-detail.tsx routes here
// instead of quietly retrying with a stale version.
export function ConflictDialog({
  open,
  onReload,
}: {
  open: boolean;
  onReload: () => void;
}) {
  return (
    <Dialog open={open}>
      <DialogContent showCloseButton={false} className="border-t-4 border-t-[oklch(0.62_0.15_75)]">
        <DialogHeader>
          <DialogTitle>Data ini sudah diubah orang lain</DialogTitle>
          <DialogDescription>
            Lead ini berubah di tempat lain sejak Anda membukanya. Perubahan yang belum tersimpan
            tidak akan ditimpakan secara otomatis — muat ulang untuk melihat versi terbaru sebelum
            melanjutkan.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" onClick={onReload}>
            Muat ulang data terkini
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
