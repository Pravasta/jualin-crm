"use client";

// The three-way branch issue #34 is built around (freeze 2.3 ketentuan
// #3). Opened ONLY after team-screen.tsx's plain deactivate attempt has
// already come back 409 membership_has_open_leads — never as an
// up-front confirmation. `openLeadCount` is always the value FROM that
// error body, never recomputed here (acceptance criterion, verbatim).
//
// Deliberately NOT a "Yakin? [Ya]/[Batal]" dialog: a lead left assigned
// to someone who can no longer log in disappears from every "My Leads"
// view AND from the "belum ter-assign" filter, because it technically
// still has an owner. That silent failure is exactly what #22 built
// on_open_leads to prevent — collapsing this into one confirm button
// throws the whole protection away.
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
import { deactivateMembership, type Member } from "@/lib/memberships";
import { globalMessage } from "@/lib/auth-errors";

type Choice = "unassign" | "reassign";

export function DeactivateMemberDialog({
  member,
  openLeadCount,
  allMembers,
  onClose,
  onDeactivated,
}: {
  member: Member | null;
  openLeadCount: number;
  allMembers: Member[];
  onClose: () => void;
  onDeactivated: () => void;
}) {
  if (!member) return null;
  return (
    <DeactivateMemberDialogContent
      key={member.id}
      member={member}
      openLeadCount={openLeadCount}
      allMembers={allMembers}
      onClose={onClose}
      onDeactivated={onDeactivated}
    />
  );
}

function DeactivateMemberDialogContent({
  member,
  openLeadCount,
  allMembers,
  onClose,
  onDeactivated,
}: {
  member: Member;
  openLeadCount: number;
  allMembers: Member[];
  onClose: () => void;
  onDeactivated: () => void;
}) {
  const [choice, setChoice] = useState<Choice | null>(null);
  const [reassignTo, setReassignTo] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const otherMembers = allMembers.filter((m) => m.id !== member.id);
  const confirmDisabled = !choice || (choice === "reassign" && !reassignTo) || loading;

  async function handleConfirm() {
    if (!choice) return;
    setError(null);
    setLoading(true);
    try {
      await deactivateMembership(member.id, {
        onOpenLeads: choice,
        reassignTo: choice === "reassign" ? reassignTo : undefined,
      });
      onDeactivated();
    } catch (err) {
      setError(globalMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="border-t-4 border-t-[oklch(0.55_0.2_25)]">
        <DialogHeader>
          <DialogTitle>Nonaktifkan {member.full_name}?</DialogTitle>
          <DialogDescription>
            {member.full_name} masih memegang <strong>{openLeadCount} lead terbuka</strong>. Pilih
            apa yang terjadi pada lead-lead tersebut sebelum melanjutkan.
          </DialogDescription>
        </DialogHeader>

        <FormErrorBanner message={error} />

        <div className="flex flex-col gap-2">
          <button
            type="button"
            onClick={() => setChoice("unassign")}
            className="rounded-md border-[1.5px] px-3 py-2.5 text-left"
            style={{
              borderColor: choice === "unassign" ? "oklch(0.56 0.19 41)" : "oklch(0.922 0 0)",
              background: choice === "unassign" ? "oklch(0.56 0.19 41 / 6%)" : "#fff",
            }}
          >
            <div className="text-[13px] font-semibold">Lepas assignment</div>
            <div className="mt-0.5 text-xs text-muted-foreground">
              Lead menjadi tanpa pemilik dan masuk daftar &ldquo;belum ter-assign&rdquo;.
            </div>
          </button>

          <button
            type="button"
            onClick={() => setChoice("reassign")}
            className="rounded-md border-[1.5px] px-3 py-2.5 text-left"
            style={{
              borderColor: choice === "reassign" ? "oklch(0.56 0.19 41)" : "oklch(0.922 0 0)",
              background: choice === "reassign" ? "oklch(0.56 0.19 41 / 6%)" : "#fff",
            }}
          >
            <div className="text-[13px] font-semibold">Pindahkan ke anggota lain</div>
            <div className="mt-0.5 text-xs text-muted-foreground">
              Semua lead terbuka ditugaskan ulang.
            </div>
            {choice === "reassign" && (
              <select
                value={reassignTo}
                onChange={(e) => setReassignTo(e.target.value)}
                onClick={(e) => e.stopPropagation()}
                className="mt-2 h-8 w-full rounded-md border border-input bg-background px-2 text-[12.5px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <option value="">Pilih anggota…</option>
                {otherMembers.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.full_name}
                  </option>
                ))}
              </select>
            )}
          </button>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            Batal
          </Button>
          <Button
            type="button"
            disabled={confirmDisabled}
            onClick={handleConfirm}
            className="bg-[oklch(0.55_0.2_25)] text-white hover:bg-[oklch(0.5_0.2_25)]"
          >
            {loading ? "Menonaktifkan…" : "Nonaktifkan"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
