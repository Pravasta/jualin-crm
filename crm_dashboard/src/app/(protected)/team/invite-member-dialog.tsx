"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FieldError } from "@/components/field-error";
import { FormErrorBanner } from "@/components/form-error-banner";
import { createInvitation, type InvitableRole } from "@/lib/invitations";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";
import { isPlanSeatLimitReached } from "@/lib/plan";

// Admin/Manager/Employee — the design's own <select> only listed the
// first two (missing Employee outright); ck_invitations_role's real
// constraint is these three (owner is rejected server-side). Corrected
// here rather than copied, same as #33's status-transition fix.
const INVITABLE_ROLES: { value: InvitableRole; label: string }[] = [
  { value: "admin", label: "Admin" },
  { value: "manager", label: "Manager" },
  { value: "employee", label: "Employee" },
];

export function InviteMemberDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}) {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<InvitableRole>("manager");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  // Set only on a REAL 403 plan_seat_limit_reached (#124) — the link to
  // /subscription only makes sense in that specific case (issue #125 AC).
  const [seatLimitReached, setSeatLimitReached] = useState(false);
  const [loading, setLoading] = useState(false);

  function reset() {
    setEmail("");
    setRole("manager");
    setFieldErrors({});
    setError(null);
    setSeatLimitReached(false);
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldErrors({});
    setSeatLimitReached(false);
    setLoading(true);
    try {
      await createInvitation(email, role);
      reset();
      onOpenChange(false);
      onCreated();
    } catch (err) {
      const fields = fieldErrorsFrom(err);
      if (Object.keys(fields).length > 0) {
        setFieldErrors(fields);
      } else {
        setError(globalMessage(err));
        setSeatLimitReached(isPlanSeatLimitReached(err));
      }
    } finally {
      setLoading(false);
    }
  }

  function goToSubscription() {
    onOpenChange(false);
    router.push("/subscription");
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Undang anggota</DialogTitle>
          <DialogDescription>Kirim undangan bergabung lewat email.</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <FormErrorBanner message={error} />
          {seatLimitReached && (
            <button
              type="button"
              onClick={goToSubscription}
              className="-mt-1.5 self-start text-[12.5px] text-accent-strong underline"
            >
              Lihat paket & pemakaian
            </button>
          )}

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="invite-email">Email</Label>
            <Input
              id="invite-email"
              type="email"
              required
              autoFocus
              placeholder="nama@usaha.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
            <FieldError message={fieldErrors.email} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="invite-role">Role</Label>
            <select
              id="invite-role"
              value={role}
              onChange={(e) => setRole(e.target.value as InvitableRole)}
              className="h-8.5 rounded-md border border-input bg-background px-2.5 text-[13px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              {INVITABLE_ROLES.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
            </select>
            <FieldError message={fieldErrors.role} />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Batal
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? "Mengirim…" : "Kirim undangan"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
