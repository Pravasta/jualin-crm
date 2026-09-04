"use client";

// One field — the name. The backend seeds every field enabled with a
// sensible Indonesian label (form.DefaultFields), so a new form is
// usable immediately; the Owner refines fields, allowlist, and copies
// the snippet from the editor it opens into.
import { useState, type FormEvent } from "react";
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
import { createForm, type Form } from "@/lib/forms";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";

export function CreateFormDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (form: Form) => void;
}) {
  const [name, setName] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  function handleOpenChange(next: boolean) {
    if (!next) {
      setName("");
      setFieldErrors({});
      setError(null);
    }
    onOpenChange(next);
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldErrors({});
    setLoading(true);
    try {
      const form = await createForm(name);
      onCreated(form);
    } catch (err) {
      // plan_upgrade_required (subscription #113) — the RACE where the
      // organization's plan closed this channel between the card
      // rendering and this click — falls into the banner branch below
      // on purpose (lib/plan.ts's isPlanUpgradeRequired names this
      // code; globalMessage(err) already renders the backend's message
      // as-is, same handling #103's delivery_not_retryable gets).
      const fields = fieldErrorsFrom(err);
      if (Object.keys(fields).length > 0) setFieldErrors(fields);
      else setError(globalMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Buat formulir</DialogTitle>
          <DialogDescription>
            Beri nama supaya mudah dikenali, misalnya &ldquo;Formulir kontak website&rdquo;.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <FormErrorBanner message={error} />

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="form-name">Nama</Label>
            <Input
              id="form-name"
              required
              autoFocus
              placeholder="Formulir kontak website"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <FieldError message={fieldErrors.name} />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => handleOpenChange(false)}>
              Batal
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? "Membuat…" : "Buat formulir"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
