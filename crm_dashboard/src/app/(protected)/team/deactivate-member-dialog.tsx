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
import { cn } from "@/lib/utils";

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
      <DialogContent className="border-t-4 border-t-destructive">
        <DialogHeader>
          <DialogTitle>Nonaktifkan {member.full_name}?</DialogTitle>
          <DialogDescription>
            {member.full_name} masih memegang <strong>{openLeadCount} lead terbuka</strong>. Pilih
            apa yang terjadi pada lead-lead tersebut sebelum melanjutkan.
          </DialogDescription>
        </DialogHeader>

        <FormErrorBanner message={error} />

        {/* A radio group, not buttons: exactly one branch must be picked
            before "Nonaktifkan" enables. The member picker sits BELOW its
            option rather than inside it — a <select> nested in a <button>
            is invalid HTML and keyboards/screen readers disagree on it
            (#165). Choosing "Batal" is the footer, not a third card. */}
        <div role="radiogroup" aria-label="Apa yang terjadi pada lead terbuka" className="flex flex-col gap-2">
          <ChoiceCard
            selected={choice === "unassign"}
            onSelect={() => setChoice("unassign")}
            title="Lepas penugasan"
            description="Lead menjadi tanpa pemilik dan masuk daftar “belum ter-assign”."
          />
          <ChoiceCard
            selected={choice === "reassign"}
            onSelect={() => setChoice("reassign")}
            title="Pindahkan ke anggota lain"
            description="Semua lead terbuka ditugaskan ulang ke satu anggota."
          />
          {choice === "reassign" && (
            <select
              value={reassignTo}
              onChange={(e) => setReassignTo(e.target.value)}
              aria-label="Pindahkan ke"
              className="h-11 w-full rounded-lg border border-input bg-card px-3 text-base outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:h-9 md:text-[14px]"
            >
              <option value="">Pilih anggota…</option>
              {otherMembers.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.full_name}
                </option>
              ))}
            </select>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            Batal
          </Button>
          <Button
            type="button"
            disabled={confirmDisabled}
            onClick={handleConfirm}
            className="bg-destructive text-white hover:bg-destructive/90"
          >
            {loading ? "Menonaktifkan…" : "Nonaktifkan"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ChoiceCard({
  selected,
  onSelect,
  title,
  description,
}: {
  selected: boolean;
  onSelect: () => void;
  title: string;
  description: string;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      className={cn(
        "min-h-11 rounded-lg border-[1.5px] px-3 py-2.5 text-left transition-colors",
        selected ? "border-primary bg-accent-tint" : "border-border bg-card hover:bg-muted"
      )}
    >
      <div className="text-[14px] font-semibold">{title}</div>
      <div className="mt-0.5 text-[12.5px] text-muted-foreground">{description}</div>
    </button>
  );
}
