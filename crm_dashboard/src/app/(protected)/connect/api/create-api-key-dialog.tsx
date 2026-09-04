"use client";

// Two stages in ONE dialog component, not two separate dialogs — the
// created secret (Aturan #21: shown exactly once, never again) never
// leaves this component's own local state. It is never passed up to
// api-keys-screen.tsx, never touches that screen's list state, never
// sits in context/localStorage/URL. Closing this dialog (setState back
// to the form stage, or unmount) is the only place the secret is ever
// discarded.
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
import { createAPIKey, type CreatedAPIKey } from "@/lib/api-keys";
import { buildCurlExample } from "@/lib/api-docs";
import { fieldErrorsFrom, globalMessage, type FieldErrors } from "@/lib/auth-errors";

type Step = "form" | "reveal";

// Same fallback api-client.ts uses for the real client — this is the
// backend the copied curl command actually needs to hit.
const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";

export function CreateAPIKeyDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called only AFTER the reveal step closes with confirmedSaved true —
   *  the list is reloaded once the user has acknowledged saving the
   *  secret, not the instant it's created. */
  onCreated: () => void;
}) {
  const [step, setStep] = useState<Step>("form");
  const [name, setName] = useState("");
  const [created, setCreated] = useState<CreatedAPIKey | null>(null);
  const [copied, setCopied] = useState(false);
  const [copiedCurl, setCopiedCurl] = useState(false);
  const [confirmedSaved, setConfirmedSaved] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  function reset() {
    setStep("form");
    setName("");
    setCreated(null); // the secret is discarded here
    setCopied(false);
    setCopiedCurl(false);
    setConfirmedSaved(false);
    setFieldErrors({});
    setError(null);
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldErrors({});
    setLoading(true);
    try {
      const result = await createAPIKey(name);
      setCreated(result);
      setStep("reveal");
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

  async function handleCopy() {
    if (!created) return;
    try {
      await navigator.clipboard.writeText(created.secret);
      setCopied(true);
    } catch {
      // Clipboard API can be unavailable (insecure context, older
      // browser) — not fatal, the secret is still visible and
      // selectable in the field below for a manual copy.
    }
  }

  // This is the ONE place in the whole product a genuinely working curl
  // example can exist — created.secret is only ever available here,
  // never again after this dialog closes. The integration docs page
  // (#49) shows the same format but can only ever fill in key_prefix, a
  // placeholder for the rest.
  async function handleCopyCurl() {
    if (!created) return;
    try {
      await navigator.clipboard.writeText(buildCurlExample(API_BASE_URL, created.secret));
      setCopiedCurl(true);
    } catch {
      // Same non-fatal reasoning as handleCopy — the command is still
      // visible and selectable below.
    }
  }

  // The single choke point every close attempt passes through — the
  // dialog's own X button, Escape, and a backdrop click all route here
  // via base-ui's onOpenChange, not just the footer button. Blocking
  // here (rather than only disabling one button) is what makes "tutup
  // yang mengharuskan konfirmasi" hold no matter how the user tries to
  // dismiss it.
  function handleOpenChange(next: boolean) {
    if (next) {
      onOpenChange(true);
      return;
    }
    if (step === "reveal" && !confirmedSaved) return; // block every close path
    const wasRevealing = step === "reveal";
    reset();
    onOpenChange(false);
    if (wasRevealing) onCreated();
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent showCloseButton={step !== "reveal"}>
        {step === "form" ? (
          <>
            <DialogHeader>
              <DialogTitle>Buat kunci baru</DialogTitle>
              <DialogDescription>
                Beri nama supaya mudah dikenali nanti, misalnya &ldquo;Website utama&rdquo;.
              </DialogDescription>
            </DialogHeader>

            <form onSubmit={handleSubmit} className="flex flex-col gap-3">
              <FormErrorBanner message={error} />

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="apikey-name">Nama</Label>
                <Input
                  id="apikey-name"
                  required
                  autoFocus
                  placeholder="Website utama"
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
                  {loading ? "Membuat…" : "Buat kunci"}
                </Button>
              </DialogFooter>
            </form>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Kunci Anda</DialogTitle>
              <DialogDescription>
                Simpan kunci ini sekarang. Setelah dialog ini ditutup, ia tidak akan ditampilkan lagi —
                tidak di layar ini, tidak di mana pun.
              </DialogDescription>
            </DialogHeader>

            <div className="flex flex-col gap-3">
              <div className="rounded-md border border-border bg-muted/40 p-2.5 font-mono text-[12.5px] break-all select-all">
                {created?.secret}
              </div>

              <Button type="button" variant="outline" onClick={handleCopy}>
                {copied ? "Tersalin" : "Salin kunci"}
              </Button>

              {/* The only honest place "disalin dan langsung bekerja" (TD §13) can
                  live — created.secret exists nowhere else once this dialog closes. */}
              <div className="flex flex-col gap-1.5">
                <Label>Contoh yang langsung bisa dipakai</Label>
                <pre className="overflow-x-auto rounded-md border border-border bg-muted/40 p-2.5 font-mono text-[11.5px] whitespace-pre-wrap">
                  {created ? buildCurlExample(API_BASE_URL, created.secret) : ""}
                </pre>
                <Button type="button" variant="outline" onClick={handleCopyCurl}>
                  {copiedCurl ? "Tersalin" : "Salin perintah"}
                </Button>
              </div>

              <label className="flex items-start gap-2 text-[12.5px] text-muted-foreground">
                <input
                  type="checkbox"
                  checked={confirmedSaved}
                  onChange={(e) => setConfirmedSaved(e.target.checked)}
                  className="mt-0.5"
                />
                Saya sudah menyimpan kunci ini di tempat yang aman.
              </label>
            </div>

            <DialogFooter>
              <Button type="button" disabled={!confirmedSaved} onClick={() => handleOpenChange(false)}>
                Selesai
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
